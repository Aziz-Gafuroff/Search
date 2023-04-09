package search

import (
	"bufio"
	"context"
	"errors"
	"log"
	"os"
	"sync"
)

var (
	ErrNoWordInFile = errors.New("no word in file")
)

// Result описывает один результат поиска.
type Result struct {
	// Фраза, которую искали
	Phrase string
	// Целиком вся строка, в которой нашли вхождение (без \r или \n в конце)
	Line string
	// Номер строки (начиная с 1), на которой нашли вхождение
	LineNum int64
	// Номер позиции (начиная с 1), на которой нашли вхождение
	ColNum int64
}

// All ищет все вхождения phrase в тестовых файлах files.
func All(ctx context.Context, phrase string, files []string) <-chan []Result {
	ch := make(chan []Result)
	var wg sync.WaitGroup

	wg.Add(len(files))

	for _, file := range files {
		go func(ctx context.Context, filename string) {
			defer wg.Done()
			fileObj, err := os.OpenFile(filename, os.O_RDONLY, 0777)
			if err != nil {
				log.Printf("file %s -> error %v", filename, err)
				return
			}

			result, err := search(ctx, fileObj, phrase, true)
			if err == ErrNoWordInFile {
				return
			}

			ch <- result

		}(ctx, file)
	}

	go func() {
		defer close(ch)
		wg.Wait()
	}()

	return ch
}

// Any ищет любое одно вхождение phrase в текстовых файлах files.
func Any(ctx context.Context, phrase string, files []string) <-chan Result {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan Result)
	wg := sync.WaitGroup{}

	msg := make(chan Result)

	wg.Add(1)
	for _, file := range files {
		go func(ctx context.Context, filename string, channel chan<- Result) {
			for {
				select {
				case <-ctx.Done():
					log.Println(ctx.Err())
					return
				default:

					fileObj, err := os.OpenFile(filename, os.O_RDONLY, 0777)
					if err != nil {
						log.Printf("file %s -> error %v", filename, err)
						return
					}

					result, err := search(ctx, fileObj, phrase, false)
					if err == ErrNoWordInFile {
						channel <- Result{}
						return
					}

					channel <- result[0]
					return
				}
			}

		}(ctx, file, msg)
	}

	winner := <-msg
	wg.Done()
	cancel()

	go func() {
		defer close(ch)
		wg.Wait()
		ch <- winner
	}()

	return ch
}

func search(ctx context.Context, file *os.File, phrase string, all bool) ([]Result, error) {
	defer file.Close()
	occurrences := make([]Result, 0, 1)

	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()

		res := searchInLine(ctx, []rune(line), []rune(phrase), all)
		if len(res) != 0 {
			for _, index := range res {
				occurrences = append(occurrences, Result{
					Phrase:  phrase,
					Line:    line,
					LineNum: int64(lineNum),
					ColNum:  int64(index + 1),
				})

				if !all {
					return occurrences, nil
				}
			}
		}

		lineNum++
	}

	if len(occurrences) == 0 {
		return nil, ErrNoWordInFile
	}

	return occurrences, nil
}

func searchInLine(ctx context.Context, line, phrase []rune, all bool) []int {
	indexes := []int{}
	i, j := 0, 0

	for i < len(line) {
		if line[i] == phrase[j] {
			if j == len(phrase)-1 {
				indexes = append(indexes, i-j)

				// Возвращаем только первое вхождение
				if !all {
					return indexes
				}

				j = -1
			}
			i++
			j++
			continue
		}

		if line[i] != phrase[j] && j == 0 {
			i++
		} else {
			j = 0
		}

	}

	return indexes
}