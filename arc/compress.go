package arc

import (
	"archiver/arc/header"
	c "archiver/compressor"
	"archiver/errtype"
	"archiver/filesystem"
	"bufio"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"
	"sync"
)

// Создает архив
func (arc Arc) Compress(paths []string) error {
	var (
		headers []header.Header
		arcFile io.WriteCloser
		files   []*header.FileItem
		err     error
	)

	filesystem.PrintPathsCheck(paths)
	if headers, err = arc.fetchHeaders(paths); err != nil {
		return errtype.ErrCompress(err)
	}
	headers = header.DropDups(headers)

	if len(headers) == 0 {
		return errtype.ErrCompress(ErrNoEntries)
	}

	arcFile, err = arc.writeArcHeader()
	if err != nil {
		return errtype.ErrCompress(
			errtype.Join(ErrWriteDirHeaders, err),
		)
	}
	arcBuf := bufio.NewWriter(arcFile)

	{
		headersPaths, f := arc.splitPathsFiles(headers)
		writers := make([]header.Writer, len(headersPaths))
		for i, p := range headersPaths {
			writers[i] = p.(header.Writer)
		}
		if err = arc.writeHeaders(writers, arcBuf); err != nil {
			return errtype.ErrCompress(err)
		}
		files = f
	}

	for i := 0; i < ncpu; i++ {
		compressor[i], err = c.NewWriter(arc.ct, compressedBuf[i], arc.cl)
		if err != nil {
			return errtype.ErrCompress(
				errtype.Join(ErrCompressorInit, err),
			)
		}
	}

	for _, fi := range files {
		if err = fi.Write(arcBuf); err != nil {
			arc.closeRemove(arcFile)
			return errtype.ErrCompress(
				errtype.Join(ErrWriteFileHeader, err),
			)
		}

		if err = arc.compressFile(fi, arcBuf); err != nil {
			arc.closeRemove(arcFile)
			return errtype.ErrCompress(
				errtype.Join(ErrCompressFile, err),
			)
		}
	}
	arcBuf.Flush()
	arcFile.Close()

	return nil
}

// Сжимает файл блоками
func (arc *Arc) compressFile(fi header.PathProvider, arcBuf io.Writer) error {
	inFile, err := os.Open(fi.PathOnDisk())
	if err != nil {
		return errtype.Join(ErrOpenFileCompress(fi.PathOnDisk()), err)
	}
	defer inFile.Close()
	inBuf := bufio.NewReader(inFile)

	var (
		wrote, read int64
		crc         uint32
		wg          = sync.WaitGroup{}
	)

	for {
		// Заполняем буферы несжатыми частями (блоками) файла
		if read, err = arc.loadUncompressedBuf(inBuf); err != nil {
			return errtype.Join(ErrReadUncompressed, err)
		}

		wg.Wait()

		if read == 0 {
			wg.Add(1)
			go arc.flushWriteBuffer(&wg, arcBuf)
			break
		}

		// Сжимаем буферы
		if err = arc.compressBuffers(); err != nil {
			return errtype.Join(ErrCompress, err)
		}

		for i := 0; i < ncpu && compressedBuf[i].Len() > 0; i++ {
			// Пишем длину сжатого блока
			length := int64(compressedBuf[i].Len())
			if err = filesystem.BinaryWrite(writeBuf, length); err != nil {
				return errtype.Join(ErrWriteBufLen, err)
			}

			crc ^= crc32.Checksum(compressedBuf[i].Bytes(), crct)

			// Пишем сжатый блок
			if wrote, err = compressedBuf[i].WriteTo(writeBuf); err != nil {
				return errtype.Join(ErrReadCompressBuf, err)
			}
			log.Println("В буфер записи записан блок размера:", wrote)
			compressor[i].Reset(compressedBuf[i])

			if writeBuf.Len() >= 8*int(bufferSize) {
				wg.Add(1)
				go arc.flushWriteBuffer(&wg, arcBuf)

				if i+1 != ncpu {
					wg.Wait()
				}
			}
		}
	}

	wg.Wait()

	// Пишем признак конца файла
	if err = filesystem.BinaryWrite(arcBuf, int64(-1)); err != nil {
		return errtype.Join(ErrWriteEOF, err)
	}
	log.Println("Записан EOF")

	// Пишем контрольную сумму
	if err = filesystem.BinaryWrite(arcBuf, crc); err != nil {
		return errtype.Join(ErrWriteCRC, err)
	}
	log.Printf("Записан CRC: %X\n", crc)

	fmt.Println(fi.PathInArc())

	return nil
}

// Загружает данные в буферы несжатых данных
func (Arc) loadUncompressedBuf(inBuf io.Reader) (read int64, err error) {
	var n int64

	for i := 0; i < ncpu && err != io.EOF; i++ {
		n, err = io.CopyN(decompressedBuf[i], inBuf, bufferSize)
		if err != nil && err != io.EOF {
			return 0, errtype.Join(ErrReadUncompressBuf, err)
		}

		read += n
	}

	return read, nil
}

// Сжимает данные в буферах несжатых данных
func (arc Arc) compressBuffers() error {
	var (
		errChan = make(chan error, ncpu)
		wg      sync.WaitGroup
	)

	for i := 0; i < ncpu && decompressedBuf[i].Len() > 0; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			_, err := decompressedBuf[i].WriteTo(compressor[i])
			if err != nil {
				errors.Join()
				errChan <- errtype.Join(ErrWriteCompressor, err)
				return
			}
			if err = compressor[i].Close(); err != nil {
				errChan <- errtype.Join(ErrCloseCompressor, err)
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		return err
	}
	return nil
}
