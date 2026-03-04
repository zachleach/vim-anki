package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type treeNode struct {
	name     string
	fullPath string
	isDir    bool
	due      int
	children []*treeNode
}

func displayDueTree(db *sql.DB, path string) {
	files := getTrackedFiles(db)
	if len(files) == 0 {
		fmt.Println("No tracked files.")
		return
	}

	// filter to path if given
	if path != "" {
		abs, _ := filepath.Abs(path)
		var filtered []string
		for _, f := range files {
			if strings.HasPrefix(f, abs+"/") || f == abs {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	if len(files) == 0 {
		fmt.Println("No tracked files in this path.")
		return
	}

	// compute due counts and find common root
	type fileInfo struct {
		path string
		due  int
	}
	var infos []fileInfo
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue // skip missing files
		}
		due := countDueInFile(db, f)
		infos = append(infos, fileInfo{path: f, due: due})
	}

	// build tree
	root := &treeNode{name: ".", isDir: true}

	for _, info := range infos {
		// determine display path
		var rel string
		if path != "" {
			abs, _ := filepath.Abs(path)
			rel, _ = filepath.Rel(abs, info.path)
		} else {
			home, _ := os.UserHomeDir()
			rel, _ = filepath.Rel(home, info.path)
			if strings.HasPrefix(rel, "..") {
				rel = info.path // use absolute if outside home
			}
		}

		parts := strings.Split(rel, "/")
		node := root
		for i, part := range parts {
			isLast := i == len(parts)-1
			found := false
			for _, child := range node.children {
				if child.name == part {
					node = child
					found = true
					break
				}
			}
			if !found {
				child := &treeNode{
					name:     part,
					fullPath: info.path,
					isDir:    !isLast,
					due:      0,
				}
				if isLast {
					child.due = info.due
				}
				node.children = append(node.children, child)
				node = child
			}
		}
	}

	// filter: only show branches with due > 0
	filterTree(root)

	if len(root.children) == 0 {
		return
	}

	fmt.Println(".")
	printTree(root, "")
}

func filterTree(node *treeNode) bool {
	if !node.isDir {
		return node.due > 0
	}

	var kept []*treeNode
	for _, child := range node.children {
		if filterTree(child) {
			kept = append(kept, child)
		}
	}
	node.children = kept
	return len(kept) > 0
}

func printTree(node *treeNode, prefix string) {
	// sort: dirs first, then files, alphabetical within each
	sort.Slice(node.children, func(i, j int) bool {
		if node.children[i].isDir != node.children[j].isDir {
			return node.children[i].isDir
		}
		return node.children[i].name < node.children[j].name
	})

	for i, child := range node.children {
		isLast := i == len(node.children)-1
		connector := "├── "
		extension := "│   "
		if isLast {
			connector = "└── "
			extension = "    "
		}

		if child.isDir {
			fmt.Printf("%s%s%s/\n", prefix, connector, child.name)
			printTree(child, prefix+extension)
		} else {
			fmt.Printf("%s%s%s %d\n", prefix, connector, child.name, child.due)
		}
	}
}

func getDueJSON(db *sql.DB, path string) string {
	files := getTrackedFiles(db)

	if path != "" {
		abs, _ := filepath.Abs(path)
		info, err := os.Stat(abs)
		if err != nil {
			return `{"total":0,"files":[]}`
		}
		if !info.IsDir() {
			// single file
			due := countDueInFile(db, abs)
			result, _ := json.Marshal(map[string]interface{}{
				"total": due,
				"files": []map[string]interface{}{
					{"path": abs, "due": due},
				},
			})
			return string(result)
		}
		// filter to directory
		var filtered []string
		for _, f := range files {
			if strings.HasPrefix(f, abs+"/") {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	total := 0
	var fileEntries []map[string]interface{}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		due := countDueInFile(db, f)
		total += due
		fileEntries = append(fileEntries, map[string]interface{}{
			"path": f,
			"due":  due,
		})
	}

	if fileEntries == nil {
		fileEntries = []map[string]interface{}{}
	}

	result, _ := json.Marshal(map[string]interface{}{
		"total": total,
		"files": fileEntries,
	})
	return string(result)
}
