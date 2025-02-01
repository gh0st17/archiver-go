package arc

import (
	"archiver/arc/internal/decompress"
	"archiver/arc/internal/generic"
	"archiver/arc/internal/header"
	"archiver/errtype"
	"io"
	"os"
)

// Выполняет распаковку архива.
//
// Открывает файл архива, пропускает магическое число и тип
// компрессора, затем обрабатывает содержимое архива, проходя
// по заголовкам разного типа. Обнаруженные заголовки
// обрабатываются соответствующими методами, а после завершения
// работы освобождаются декомпрессоры.
func (arc Arc) Decompress() error {
	arcFile, err := os.OpenFile(arc.path, os.O_RDONLY, 0644)
	if err != nil {
		return errtype.ErrDecompress(errtype.Join(ErrOpenArc, err))
	}
	defer arcFile.Close()

	if err := generic.LoadDict(arc.RestoreParams); err != nil {
		return errtype.ErrDecompress(err)
	}

	if err := generic.ProcessHeaders(arcFile, headerLen, arc.restoreHandler); err != nil {
		return errtype.ErrDecompress(err)
	}

	// Сброс декомпрессоров перед новым вызовом этой функции
	generic.ResetDecomp()

	return nil
}

// Обработчик заголовков архива для распаковки
func (arc Arc) restoreHandler(typ header.HeaderType, arcFile io.ReadSeekCloser) (err error) {
	switch typ {
	case header.File:
		err = decompress.RestoreFile(arcFile, arc.RestoreParams)
	case header.Symlink:
		err = decompress.RestoreSym(arcFile, arc.RestoreParams)
	default:
		return ErrHeaderType
	}
	if err != nil && err != io.EOF {
		return errtype.ErrDecompress(err)
	}

	return nil
}
