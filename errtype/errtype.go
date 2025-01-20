package errtype

import (
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
)

type Error struct {
	text string
	code int // Код завершения после вывода ошибки
}

// Перевод и форматирование встроенных ошибок
func (e Error) Error() string {
	return e.text
}

func stdErrToRussian(err error) error {
	if err == nil {
		return nil
	} else if _, ok := err.(*Error); ok {
		return err
	}

	switch {
	case errors.Is(err, gzip.ErrHeader) || errors.Is(err, zlib.ErrHeader):
		return errors.New("ошибка заголовка сжатых данных")
	case errors.Is(err, gzip.ErrChecksum) || errors.Is(err, zlib.ErrChecksum):
		return errors.New("неверная контрольная сумма")
	case errors.Is(err, os.ErrPermission):
		return errors.New(fmt.Sprint("нет доступа", err))
	case errors.Is(err, os.ErrExist):
		return errors.New("файл уже существует")
	case errors.Is(err, os.ErrNotExist):
		return errors.New("файл не существует")
	case errors.Is(err, io.EOF):
		return errors.New("достигнут конец файла")
	case errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New("неожиданный конец файла")
	default:
		return err
	}
}

// Возвращает общую ошибку времени выполнения
func ErrRuntime(text string) error {
	return &Error{
		text: text,
		code: 1,
	}
}

// Возвращает ошибки при сжатии
func ErrCompress(text string) error {
	return &Error{
		text: text,
		code: 2,
	}
}

// Возвращает ошибки при распаковке
func ErrDecompress(text string) error {
	return &Error{
		text: text,
		code: 3,
	}
}

// Возвращает ошибки при проверке целостности
func ErrIntegrity(text string) error {
	return &Error{
		text: text,
		code: 4,
	}
}

// Объединяет описание ошибок в цепочку
//
// Копирует логику [errors.Join], но делает
// это в одну строку
func Join(errs ...error) error {
	n := 0
	for _, err := range errs {
		if err != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}

	var e error
	for _, err := range errs {
		if err == nil {
			continue
		} else if _, ok := err.(*Error); !ok {
			err = stdErrToRussian(err)
		}

		if e == nil {
			e = err
		} else if err != nil {
			e = fmt.Errorf("%v: %v", e, err)
		}
	}
	return e
}

// Обработчик ошибок
func HandleError(err error) {
	fmt.Println(err)
	if e, ok := err.(*Error); ok {
		os.Exit(e.code)
	} else {
		os.Exit(-1)
	}
}
