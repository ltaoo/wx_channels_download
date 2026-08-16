package youtube

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

const (
	youtube_player_solver_cache_version  = 1
	youtube_player_solver_cache_max_size = 32 << 20
	youtube_player_solver_memory_entries = 4
)

type player_solver_cache_record struct {
	Version            int    `json:"version"`
	PlayerSHA256       string `json:"player_sha256"`
	PreprocessedSHA256 string `json:"preprocessed_sha256"`
	PreprocessedPlayer string `json:"preprocessed_player"`
}

type compiled_player_solver struct {
	program            *goja.Program
	preprocessed_bytes int
}

type player_solver_flight struct {
	done    chan struct{}
	entry   *compiled_player_solver
	source  string
	err     error
	waiters int
}

type player_solver_registry struct {
	mu      sync.Mutex
	entries map[string]*compiled_player_solver
	order   []string
	flights map[string]*player_solver_flight
	limit   int
}

var youtube_player_solvers = new_player_solver_registry(youtube_player_solver_memory_entries)

func new_player_solver_registry(limit int) *player_solver_registry {
	if limit < 1 {
		limit = 1
	}
	return &player_solver_registry{
		entries: make(map[string]*compiled_player_solver),
		flights: make(map[string]*player_solver_flight),
		limit:   limit,
	}
}

func (r *player_solver_registry) get_or_build(
	ctx context.Context,
	key string,
	build func() (*compiled_player_solver, string, error),
) (*compiled_player_solver, string, error) {
	r.mu.Lock()
	if entry := r.entries[key]; entry != nil {
		r.touch_locked(key)
		r.mu.Unlock()
		return entry, "memory", nil
	}
	if flight := r.flights[key]; flight != nil {
		flight.waiters++
		r.mu.Unlock()
		select {
		case <-flight.done:
			if flight.err != nil {
				return nil, "shared_" + flight.source, flight.err
			}
			return flight.entry, "shared_" + flight.source, nil
		case <-ctx.Done():
			return nil, "shared", ctx.Err()
		}
	}
	flight := &player_solver_flight{done: make(chan struct{})}
	r.flights[key] = flight
	r.mu.Unlock()

	entry, source, err := build()

	r.mu.Lock()
	if err == nil && entry != nil {
		r.put_locked(key, entry)
	}
	flight.entry = entry
	flight.source = source
	flight.err = err
	delete(r.flights, key)
	close(flight.done)
	r.mu.Unlock()
	return entry, source, err
}

func (r *player_solver_registry) touch_locked(key string) {
	for index, existing := range r.order {
		if existing != key {
			continue
		}
		copy(r.order[index:], r.order[index+1:])
		r.order[len(r.order)-1] = key
		return
	}
	r.order = append(r.order, key)
}

func (r *player_solver_registry) put_locked(key string, entry *compiled_player_solver) {
	if _, exists := r.entries[key]; exists {
		r.entries[key] = entry
		r.touch_locked(key)
		return
	}
	r.entries[key] = entry
	r.order = append(r.order, key)
	for len(r.order) > r.limit {
		oldest := r.order[0]
		r.order = r.order[1:]
		delete(r.entries, oldest)
	}
}

func player_solver_key(player_js string) string {
	digest := sha256.Sum256([]byte(player_js))
	return hex.EncodeToString(digest[:])
}

func player_solver_cache_relative_path(key string) (string, error) {
	if len(key) != sha256.Size*2 {
		return "", fmt.Errorf("invalid youtube player solver cache key")
	}
	if _, err := hex.DecodeString(key); err != nil {
		return "", fmt.Errorf("invalid youtube player solver cache key: %w", err)
	}
	return filepath.ToSlash(filepath.Join("jsc", "v1", key+".json")), nil
}

func preprocessed_player_digest(preprocessed string) string {
	digest := sha256.Sum256([]byte(preprocessed))
	return hex.EncodeToString(digest[:])
}

func (c *Client) read_cached_player_solver(key string) (string, bool, error) {
	if c == nil || c.Cache == nil || !c.Cache.Enabled() {
		return "", false, nil
	}
	relative_path, err := player_solver_cache_relative_path(key)
	if err != nil {
		return "", false, err
	}
	file_info, err := c.Cache.Stat(relative_path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("stat youtube player solver cache: %w", err)
	}
	if !file_info.Mode().IsRegular() || file_info.Size() <= 0 || file_info.Size() > youtube_player_solver_cache_max_size {
		return "", false, fmt.Errorf("invalid youtube player solver cache size: %d", file_info.Size())
	}
	data, err := c.Cache.Read(relative_path)
	if err != nil {
		return "", false, fmt.Errorf("read youtube player solver cache: %w", err)
	}
	var record player_solver_cache_record
	if err := json.Unmarshal(data, &record); err != nil {
		return "", false, fmt.Errorf("decode youtube player solver cache: %w", err)
	}
	if record.Version != youtube_player_solver_cache_version || record.PlayerSHA256 != key {
		return "", false, errors.New("youtube player solver cache metadata does not match")
	}
	if strings.TrimSpace(record.PreprocessedPlayer) == "" || preprocessed_player_digest(record.PreprocessedPlayer) != record.PreprocessedSHA256 {
		return "", false, errors.New("youtube player solver cache checksum does not match")
	}
	return record.PreprocessedPlayer, true, nil
}

func (c *Client) write_cached_player_solver(key, preprocessed string) error {
	if c == nil || c.Cache == nil || !c.Cache.Enabled() {
		return nil
	}
	relative_path, err := player_solver_cache_relative_path(key)
	if err != nil {
		return err
	}
	record := player_solver_cache_record{
		Version:            youtube_player_solver_cache_version,
		PlayerSHA256:       key,
		PreprocessedSHA256: preprocessed_player_digest(preprocessed),
		PreprocessedPlayer: preprocessed,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode youtube player solver cache: %w", err)
	}
	if len(data) > youtube_player_solver_cache_max_size {
		return fmt.Errorf("youtube player solver cache is too large: %d", len(data))
	}
	if err := c.Cache.Write(relative_path, data); err != nil {
		return fmt.Errorf("write youtube player solver cache: %w", err)
	}
	return nil
}

func (c *Client) remove_cached_player_solver(key string) error {
	if c == nil || c.Cache == nil || !c.Cache.Enabled() {
		return nil
	}
	relative_path, err := player_solver_cache_relative_path(key)
	if err != nil {
		return err
	}
	return c.Cache.Remove(relative_path)
}

func (r *player_resolver) solve_cached_player_challenges(player_js, challenge_type string, challenges []string) (map[string]string, error) {
	key := player_solver_key(player_js)
	prepare_started := time.Now()
	entry, cache_layer, err := youtube_player_solvers.get_or_build(r.ctx, key, func() (*compiled_player_solver, string, error) {
		preprocessed, cached, cache_err := r.client.read_cached_player_solver(key)
		if cache_err != nil {
			r.log_player_solver_cache_warning("read", key, cache_err)
			if remove_err := r.client.remove_cached_player_solver(key); remove_err != nil {
				r.log_player_solver_cache_warning("remove", key, remove_err)
			}
		}
		if cached {
			program, compile_err := compile_preprocessed_player(key, preprocessed)
			if compile_err == nil {
				return &compiled_player_solver{program: program, preprocessed_bytes: len(preprocessed)}, "disk", nil
			}
			r.log_player_solver_cache_warning("compile", key, compile_err)
			if remove_err := r.client.remove_cached_player_solver(key); remove_err != nil {
				r.log_player_solver_cache_warning("remove", key, remove_err)
			}
		}

		preprocessed, preprocess_err := preprocess_player_with_goja(r.ctx, player_js)
		if preprocess_err != nil {
			return nil, "miss", preprocess_err
		}
		program, compile_err := compile_preprocessed_player(key, preprocessed)
		if compile_err != nil {
			return nil, "miss", compile_err
		}
		if write_err := r.client.write_cached_player_solver(key, preprocessed); write_err != nil {
			r.log_player_solver_cache_warning("write", key, write_err)
		}
		return &compiled_player_solver{program: program, preprocessed_bytes: len(preprocessed)}, "miss", nil
	})
	r.log_player_solver_preparation(cache_layer, key, entry, prepare_started, err)
	if err != nil {
		return nil, err
	}

	solve_started := time.Now()
	solved, err := solve_preprocessed_player_with_goja(r.ctx, entry.program, challenge_type, challenges)
	r.log_challenge_solver_result(challenge_type, "preprocessed_ejs_goja", len(challenges), solve_started, err)
	return solved, err
}

func compile_preprocessed_player(key, preprocessed string) (*goja.Program, error) {
	name := "youtube-player.js"
	if len(key) >= 12 {
		name = "youtube-player-" + key[:12] + ".js"
	}
	program, err := goja.Compile(name, preprocessed, false)
	if err != nil {
		return nil, fmt.Errorf("compile preprocessed youtube player: %w", err)
	}
	return program, nil
}
