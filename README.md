# archiver-go

<p align="center">
  Архиватор на базе встроенной библиотеки Golang
</p>

<p align="center">
  <a href="https://github.com/gh0st17/archiver-go/releases/latest"><img src="https://img.shields.io/github/v/release/gh0st17/archiver-go?style=plastic"></a>
  <img src="https://img.shields.io/badge/license-MIT-blue?style=plastic">
  <img src="https://tokei.rs/b1/github/gh0st17/archiver-go?category=code">
</p>

# О проекте

Данная программа реализована в целях изучения языка Golang.

# Возможности

- Конкурентное сжатие и распаковка с возможностью параллелизма
- Несколько алгоритмов для сжатия
- Просмотр содержимого архива в виде списка или детального отчета
- Проверка целостности данных в архиве и распаковка с учетом проверки
- Поддержка символических ссылок

# Справка по использованию

```
Сжатие:     archiver [Флаги] <путь до архива> <список директории, файлов для сжатия>
Распаковка: archiver [-o <путь к директории для распаковки>] <путь до архива>
Просмотр:   archiver [-l | -s] <путь до архива>

Флаги:
  -L int
    	Уровень сжатия от -2 до 9 (Не применяется для LZW)
    	 -2 -- Использовать только сжатие по Хаффману
    	 -1 -- Уровень сжатия по умолчанию (6)
    	  0 -- Без сжатия
    	1-9 -- Произвольная степень сжатия (default -1)
  -V	Печать номера версии и выход
  -c string
    	Тип компрессора: GZip, LZW, ZLib, Flate (default "gzip")
  -dict string
    	Путь к файлу словаря
    	Файл словаря представляет собой набор на часто встречающихся
    	фрагментов данных, которые можно использовать для улучшения
    	сжатия. При декомпрессии необходимо использовать тот же
    	словарь для восстановления данных.
  -f	Автоматически заменять файлы при распаковке без подтверждения
  -help
    	Показать эту помощь
  -integ
    	Проверка целостности данных в архиве
  -l	Печать списка файлов и выход
  -log
    	Печатать логи
  -mstat
    	Печать статистики использования ОЗУ после выполнения
  -o string
    	Путь к директории для распаковки
  -s	Печать информации о сжатии и выход (игнорирует -l)
  -xinteg
    	Распаковка с учетом проверки целостности данных в архиве
```
