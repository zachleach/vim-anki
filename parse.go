package main

import (
	"os"
	"strings"
)

type Chunk struct {
	QuestionLine string
	FullText     string
}

func parseChunks(filePath string) ([]Chunk, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var chunks []Chunk
	var current []string

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, ">\t") {
			if len(current) > 0 {
				chunks = append(chunks, buildChunk(current))
			}
			current = []string{line}
		} else if len(current) > 0 {
			current = append(current, line)
		}
	}

	if len(current) > 0 {
		chunks = append(chunks, buildChunk(current))
	}

	return chunks, nil
}

func buildChunk(lines []string) Chunk {
	// trim trailing blank lines
	for len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return Chunk{
		QuestionLine: strings.TrimRight(strings.TrimPrefix(lines[0], ">\t"), " \t"),
		FullText:     strings.Join(lines, "\n"),
	}
}

func hasQuestionLines(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, ">\t") {
			return true
		}
	}
	return false
}
