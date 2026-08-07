package config

import (
	"time"

	"github.com/spf13/viper"
)

// SubViper wraps a viper.Viper to provide scoped, read-only access within a config
// namespace. Plugins receive a SubViper scoped to their namespace (e.g. "channels",
// "69shuba") so they can only read their own keys.
type SubViper struct {
	v *viper.Viper
}

func NewSubViper(v *viper.Viper) *SubViper {
	if v == nil {
		v = viper.New()
	}
	return &SubViper{v: v}
}

func (s *SubViper) GetString(key string) string   { return s.v.GetString(key) }
func (s *SubViper) GetBool(key string) bool       { return s.v.GetBool(key) }
func (s *SubViper) GetInt(key string) int         { return s.v.GetInt(key) }
func (s *SubViper) GetInt64(key string) int64     { return s.v.GetInt64(key) }
func (s *SubViper) GetFloat64(key string) float64 { return s.v.GetFloat64(key) }
func (s *SubViper) GetDuration(key string) time.Duration { return s.v.GetDuration(key) }
func (s *SubViper) GetStringSlice(key string) []string   { return s.v.GetStringSlice(key) }
func (s *SubViper) GetStringMap(key string) map[string]interface{} {
	return s.v.GetStringMap(key)
}
func (s *SubViper) IsSet(key string) bool { return s.v.IsSet(key) }
