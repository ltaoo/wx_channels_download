package download

import (
	"path/filepath"
	"strings"
)

func CloneFileNodes(files []FileNode) []FileNode {
	if len(files) == 0 {
		return nil
	}
	out := make([]FileNode, len(files))
	for i := range files {
		out[i] = files[i]
		out[i].Children = CloneFileNodes(files[i].Children)
	}
	return out
}

func SingleFileNodes(path string, size int64, status string) []FileNode {
	name := strings.TrimSpace(filepath.Base(path))
	if name == "." || name == string(filepath.Separator) {
		name = strings.TrimSpace(path)
	}
	if name == "" {
		name = "download"
	}
	return []FileNode{{
		Name:       name,
		Path:       filepath.ToSlash(name),
		OutputPath: path,
		Type:       FileNodeTypeFile,
		Size:       size,
		Status:     status,
	}}
}

func FileNodesWithOutputPath(root string, files []FileNode) []FileNode {
	out := CloneFileNodes(files)
	fillFileNodeOutputPath(root, out)
	return out
}

func fillFileNodeOutputPath(root string, files []FileNode) {
	for i := range files {
		if strings.TrimSpace(files[i].OutputPath) == "" {
			rel := strings.TrimSpace(files[i].Path)
			if rel == "" {
				rel = strings.TrimSpace(files[i].Name)
			}
			if rel != "" {
				if filepath.IsAbs(rel) {
					files[i].OutputPath = filepath.Clean(rel)
				} else if strings.TrimSpace(root) != "" {
					files[i].OutputPath = filepath.Join(root, filepath.FromSlash(rel))
				}
			}
		}
		if len(files[i].Children) > 0 {
			fillFileNodeOutputPath(root, files[i].Children)
		}
	}
}

func FileNodesSize(files []FileNode) int64 {
	var size int64
	for _, file := range files {
		if strings.EqualFold(file.Type, FileNodeTypeDir) || len(file.Children) > 0 {
			size += FileNodesSize(file.Children)
			continue
		}
		size += file.Size
	}
	return size
}

func FileNodesCount(files []FileNode) int {
	count := 0
	for _, file := range files {
		if strings.EqualFold(file.Type, FileNodeTypeDir) || len(file.Children) > 0 {
			count += FileNodesCount(file.Children)
			continue
		}
		count++
	}
	return count
}
