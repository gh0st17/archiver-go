package arc

import (
	"archiver/arc/header"
	c "archiver/compressor"
	"archiver/filesystem"
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

func (arc Arc) prepareArcFile() (arcFile *os.File, err error) {
	arcFile, err = os.Open(arc.arcPath)
	if err != nil {
		return nil, fmt.Errorf("prepareArcFile: can't open archive: %v", err)
	}

	pos, err := arcFile.Seek(arc.dataOffset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("prepareArcFile: can't seek: %v", err)
	}
	log.Println("Prepare: pos set to ", pos)

	return arcFile, nil
}

// Распаковывает архив
func (arc Arc) Decompress(outputDir string, integ bool) error {
	headers, err := arc.readHeaders()
	if err != nil {
		return fmt.Errorf("decompress: can't read headers: %v", err)
	}

	arcFile, err := arc.prepareArcFile()
	if err != nil {
		return err
	}
	defer arcFile.Close()

	// 	Создаем файлы и директории
	var (
		outPath          string
		skipLen, dataPos int64
		firstFileIdx     int
	)

	if firstFileIdx, err = arc.findFileIdx(headers, outputDir); err != nil {
		return err
	}

	if firstFileIdx >= len(headers) { // В архиве нет файлов
		return nil
	}

	for _, h := range headers[firstFileIdx:] {
		fi := h.(*header.FileItem)
		outPath = filepath.Join(outputDir, fi.Path())
		if err = filesystem.CreatePath(filepath.Dir(outPath)); err != nil {
			return fmt.Errorf("decompress: can't create path '%s': %v", outPath, err)
		}

		skipLen = int64(len(fi.Path())) + 32
		if dataPos, err = arcFile.Seek(skipLen, io.SeekCurrent); err != nil {
			return err
		}
		log.Println("Skipped", skipLen, "bytes of file header, read from pos:", dataPos)

		if _, err := os.Stat(outPath); err == nil && !arc.replaceAll {
			if arc.replaceInput(outPath, arcFile) {
				continue
			}
		}

		if integ { // --xinteg
			arc.checkCRC(fi, arcFile)

			if fi.IsDamaged() {
				fmt.Printf("Пропускаю повежденный '%s'\n", fi.Path())
				continue
			} else {
				arcFile.Seek(dataPos, io.SeekStart)
				log.Println("Set to pos:", dataPos+skipLen)
			}
		}

		if err = arc.decompressFile(fi, arcFile, outPath); err != nil {
			return fmt.Errorf("decompress: %v", err)
		}

		if dataPos, err = arcFile.Seek(4, io.SeekCurrent); err != nil {
			return err
		}
		log.Println("Skipped CRC, new arcFile pos:", dataPos)

		if err = os.Chtimes(outPath, fi.Atim(), fi.Mtim()); err != nil {
			return err
		}

		if fi.IsDamaged() {
			fmt.Printf("%s: CRC сумма не совпадает\n", outPath)
		} else {
			fmt.Println(outPath)
		}
	}

	return nil
}

// Возвращает первый индекс, указывающий на файл.
// Воссоздает пути для распаковки если они не существуют.
func (Arc) findFileIdx(headers []header.Header, outputDir string) (int, error) {
	var (
		di           *header.DirItem
		ok           bool
		firstFileIdx int
	)

	for {
		if di, ok = headers[firstFileIdx].(*header.DirItem); !ok {
			break
		}

		outPath := filepath.Join(outputDir, di.Path())
		if _, err := os.Stat(outPath); errors.Is(err, os.ErrNotExist) {
			if err = filesystem.CreatePath(outPath); err != nil {
				return 0, fmt.Errorf("findFileIdx: can't create path '%s': %v", outPath, err)
			}
			if err = os.Chtimes(outPath, di.Atim(), di.Mtim()); err != nil {
				return 0, err
			}
		}

		fmt.Println(outPath)
		firstFileIdx++
		if firstFileIdx >= len(headers) {
			break
		}
	}

	return firstFileIdx, nil
}

func (arc Arc) replaceInput(outPath string, arcFile io.ReadSeeker) bool {
	var input rune
	stdin := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Файл '%s' существует, заменить? [(Д)а/(Н)ет/(В)се]: ", outPath)
		input, _, _ = stdin.ReadRune()

		switch input {
		case 'A', 'a', 'В', 'в':
			arc.replaceAll = true
		case 'Y', 'y', 'Д', 'д':
		case 'N', 'n', 'Н', 'н':
			arc.skipFile(arcFile)
			return true
		default:
			stdin.ReadString('\n')
			continue
		}
		break
	}

	return false
}

// Распаковывает файл
func (arc Arc) decompressFile(fi *header.FileItem, arcFile io.ReadSeeker, outPath string) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("decompressFile: %v", err)
	}
	defer outFile.Close()

	// Если размер файла равен 0, то пропускаем запись
	if fi.UcSize() == 0 {
		pos, _ := arcFile.Seek(8, io.SeekCurrent) // Пропуск признака конца файла
		log.Println("Empty size, set to pos:", pos)
		return nil
	}

	var (
		n   int64
		crc = fi.CRC()
	)
	for n != -1 {
		pos, _ := arcFile.Seek(0, io.SeekCurrent)
		log.Println("loadCompBuf: start reading from pos: ", pos)
		if n, err = arc.loadCompressedBuf(arcFile); err != nil {
			return fmt.Errorf("decompressFile: %v", err)
		}

		for i := 0; i < ncpu && compressedBuf[i].Len() > 0; i++ {
			crc ^= crc32.Checksum(compressedBuf[i].Bytes(), crct)
		}

		if err = arc.decompressBuffers(); err != nil {
			return fmt.Errorf("decompressFile: %v", err)
		}

		for i := 0; i < ncpu && decompressedBuf[i].Len() > 0; i++ {
			decompressedBuf[i].WriteTo(outFile)
		}
	}
	fi.SetDamaged(crc != 0)

	return nil
}

// Загружает данные в буферы сжатых данных
func (Arc) loadCompressedBuf(r io.Reader) (read int64, err error) {
	var n, bufferSize int64

	for i := 0; i < ncpu; i++ {
		if err = binary.Read(r, binary.LittleEndian, &bufferSize); err != nil {
			return 0, fmt.Errorf("loadCompressedBuf: can't binary read: %v", err)
		}

		if bufferSize == -1 {
			log.Println("Read EOF")
			return -1, nil
		}
		log.Println("Read length of compressed data:", bufferSize)

		lim := io.LimitReader(r, bufferSize)
		if n, err = compressedBuf[i].ReadFrom(lim); err != nil {
			return 0, fmt.Errorf("loadCompressedBuf: can't read: %v", err)
		}

		read += n
	}

	return read, nil
}

// Распаковывает данные в буферах сжатых данных
func (arc Arc) decompressBuffers() error {
	var (
		errChan = make(chan error, ncpu)
		wg      sync.WaitGroup
	)

	for i := 0; i < ncpu && compressedBuf[i].Len() > 0; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			decompressor := c.NewReader(arc.ct, compressedBuf[i])
			defer decompressor.Close()
			_, err := decompressedBuf[i].ReadFrom(decompressor)
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		return fmt.Errorf("decompressBuf: %v", err)
	}

	return nil
}

// Пропускает файл в arcFile
func (arc Arc) skipFile(arcFile io.ReadSeeker) (read header.Size, err error) {
	var bufferSize header.Size

	for bufferSize != -1 {
		read += bufferSize
		if _, err = arcFile.Seek(int64(bufferSize), io.SeekCurrent); err != nil {
			return 0, err
		}

		if err = binary.Read(arcFile, binary.LittleEndian, &bufferSize); err != nil {
			return 0, fmt.Errorf("skipFile: can't binary read: %v", err)
		}
	}

	if _, err = arcFile.Seek(4, io.SeekCurrent); err != nil {
		return 0, err
	}

	return read, nil
}
