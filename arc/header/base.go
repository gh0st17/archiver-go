package header

import (
	"archiver/errtype"
	"archiver/filesystem"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Base struct {
	pathOnDisk string    // Путь к элементу на диске
	pathInArc  string    // Путь к элементу в архиве
	atim       time.Time // Последнее время доступа к элементу
	mtim       time.Time // Последнее время измения элемента
}

// Возвращает путь до элемента
func (b Base) PathOnDisk() string { return b.pathOnDisk }
func (b Base) PathInArc() string  { return b.pathInArc }

func NewBase(pathOnDisk string, atim, mtim time.Time) Base {
	return Base{pathOnDisk, filesystem.Clean(pathOnDisk), atim, mtim}
}

// Сериализует в себя данные из r
func (b *Base) Read(r io.Reader) error {
	var (
		err                error
		length             int16
		filePathBytes      []byte
		unixMtim, unixAtim int64
	)

	// Читаем размер строки имени файла или директории
	if err = binary.Read(r, binary.LittleEndian, &length); err != nil {
		return err
	}

	if length < 1 || length >= 1024 {
		return errtype.ErrRuntime(
			fmt.Errorf("некорректная длина (%d) пути элемента", length), nil,
		)
	}

	// Читаем имя файла
	filePathBytes = make([]byte, length)
	if _, err := io.ReadFull(r, filePathBytes); err != nil {
		return err
	}

	// Читаем время модификации
	if err = binary.Read(r, binary.LittleEndian, &unixMtim); err != nil {
		return err
	}

	// Читаем время доступа
	if err = binary.Read(r, binary.LittleEndian, &unixAtim); err != nil {
		return err
	}

	mtim, atim := time.Unix(unixMtim, 0), time.Unix(unixAtim, 0)
	*b = NewBase(string(filePathBytes), mtim, atim)

	return nil
}

// Сериализует данные полей в писатель w
func (b *Base) Write(w io.Writer) (err error) {
	// Пишем длину строки имени файла или директории
	if err = binary.Write(w, binary.LittleEndian, int16(len(b.pathInArc))); err != nil {
		return err
	}

	// Пишем имя файла или директории
	if err = binary.Write(w, binary.LittleEndian, []byte(b.pathInArc)); err != nil {
		return err
	}

	atime, mtime := b.atim.Unix(), b.mtim.Unix()

	// Пишем время модификации
	if err = binary.Write(w, binary.LittleEndian, mtime); err != nil {
		return err
	}

	// Пишем имя время доступа
	if err = binary.Write(w, binary.LittleEndian, atime); err != nil {
		return err
	}

	return nil
}

func (b Base) RestoreTime(outDir string) error {
	outDir = filepath.Join(outDir, b.pathOnDisk)
	if err := os.Chtimes(outDir, b.atim, b.mtim); err != nil {
		return err
	}

	return nil
}

func (b Base) createPath(outDir string) error {
	return filesystem.CreatePath(outDir)
}
