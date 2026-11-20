package application

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Chromium's Blockfile layout is defined in net/disk_cache/blockfile/
// disk_format.h, disk_format_base.h and addr.h. Entries share data_N files;
// moving individual files would corrupt unrelated entries in the same cache.
const liteapp_blockfile_index_header_size = 368

func liteapp_blockfile_cache_matches(cache_dir string) (bool, error) {
	index, err := open_liteapp_cache_file(filepath.Join(cache_dir, "index"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer index.Close()
	var header [liteapp_blockfile_index_header_size]byte
	if _, err := io.ReadFull(index, header[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return false, nil
		}
		return false, err
	}
	u32 := binary.LittleEndian.Uint32
	version := u32(header[4:8])
	if u32(header[:4]) != 0xc103cac3 || (version != 0x20000 && version != 0x20001 && version != 0x30000) {
		return false, nil
	}
	table_len := u32(header[28:32])
	if table_len == 0 {
		table_len = 0x10000
	}
	if table_len > 1<<20 {
		return false, fmt.Errorf("invalid Blockfile index table length: %d", table_len)
	}
	table := make([]byte, int(table_len)*4)
	if _, err := io.ReadFull(index, table); err != nil {
		return false, err
	}
	reader := liteapp_blockfile_reader{dir: cache_dir, files: make(map[uint32]*os.File)}
	defer reader.close()
	visited := make(map[uint32]bool)
	for offset := 0; offset < len(table); offset += 4 {
		for addr := u32(table[offset : offset+4]); addr != 0; {
			if visited[addr] {
				return false, fmt.Errorf("duplicate or cyclic Blockfile entry: %08x", addr)
			}
			if len(visited) >= 1<<20 {
				return false, fmt.Errorf("too many Blockfile entries")
			}
			visited[addr] = true
			// EntryStore always lives in one to four 256-byte blocks.
			if (addr>>28)&7 != 2 {
				return false, fmt.Errorf("invalid Blockfile entry address: %08x", addr)
			}
			entry, err := reader.read(addr, 256*int(1+(addr>>24)&3))
			if err != nil {
				return false, err
			}
			if u32(entry[20:24]) == 0 { // ENTRY_NORMAL
				key_len := u32(entry[32:36])
				long_key := u32(entry[36:40])
				if key_len > 0 && key_len <= 16*1024 {
					var key []byte
					if long_key != 0 {
						key, err = reader.read(long_key, int(key_len)+1)
						if err != nil {
							return false, err
						}
					} else if int(key_len) < len(entry)-96 {
						key = entry[96 : 96+int(key_len)+1]
					}
					if len(key) == int(key_len)+1 && key[key_len] == 0 && liteapp_sdk_cache_key_matches(key[:key_len]) {
						return true, nil
					}
				}
			}
			addr = u32(entry[4:8])
		}
	}
	return false, nil
}

func open_liteapp_cache_file(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular cache file: %s", path)
	}
	return os.Open(path)
}

type liteapp_blockfile_reader struct {
	dir   string
	files map[uint32]*os.File
}

func (r *liteapp_blockfile_reader) close() {
	for _, file := range r.files {
		file.Close()
	}
}

func (r *liteapp_blockfile_reader) read(addr uint32, length int) ([]byte, error) {
	if addr&0x80000000 == 0 || ((addr>>28)&7 != 0 && addr&0x0c000000 != 0) {
		return nil, fmt.Errorf("invalid Blockfile address: %08x", addr)
	}
	file_type := (addr >> 28) & 7
	file_number := (addr >> 16) & 255
	name := fmt.Sprintf("data_%d", file_number)
	var offset int64
	var block_size uint32
	if file_type == 0 {
		file_number = addr & 0x0fffffff
		name = fmt.Sprintf("f_%06x", file_number)
	} else {
		switch file_type {
		case 2:
			block_size = 256
		case 3:
			block_size = 1024
		case 4:
			block_size = 4096
		default:
			return nil, fmt.Errorf("unsupported Blockfile address: %08x", addr)
		}
		if length > int(block_size*(1+(addr>>24)&3)) {
			return nil, fmt.Errorf("Blockfile key exceeds allocated blocks")
		}
		offset = 8192 + int64(addr&0xffff)*int64(block_size)
	}
	file_id := (file_type << 28) | file_number
	file := r.files[file_id]
	if file == nil {
		var err error
		file, err = open_liteapp_cache_file(filepath.Join(r.dir, name))
		if err != nil {
			return nil, err
		}
		r.files[file_id] = file
		if file_type != 0 {
			var header [16]byte
			if _, err := file.ReadAt(header[:], 0); err != nil {
				return nil, err
			}
			u32 := binary.LittleEndian.Uint32
			if u32(header[:4]) != 0xc104cac3 || u32(header[4:8]) != 0x20000 || uint32(binary.LittleEndian.Uint16(header[8:10])) != file_number || u32(header[12:16]) != block_size {
				return nil, fmt.Errorf("invalid Blockfile header: %s", name)
			}
		}
	}
	data := make([]byte, length)
	_, err := file.ReadAt(data, offset)
	return data, err
}

func move_liteapp_blockfile_cache(cache_dir string) (string, error) {
	for _, directory := range []string{filepath.Dir(cache_dir), cache_dir} {
		valid, err := liteapp_cache_directory(directory)
		if err != nil {
			return "", err
		}
		if !valid {
			return "", fmt.Errorf("invalid cache directory: %s", directory)
		}
	}
	matches, err := liteapp_blockfile_cache_matches(cache_dir)
	if err != nil || !matches {
		return "", err
	}
	backup_dir, err := new_liteapp_cache_backup(filepath.Dir(filepath.Dir(cache_dir)))
	if err != nil {
		return "", err
	}
	// A single rename keeps the index, shared blocks and external streams intact.
	// WeChat creates a fresh Cache_Data directory on its next launch.
	if err := os.Rename(cache_dir, filepath.Join(backup_dir, "Cache_Data")); err != nil {
		return "", fmt.Errorf("move %s to %s: %w", cache_dir, backup_dir, err)
	}
	return backup_dir, nil
}
