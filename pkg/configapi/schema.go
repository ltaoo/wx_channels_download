package configapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type ValueType string

const (
	TypeString      ValueType = "string"
	TypeBool        ValueType = "boolean"
	TypeInt         ValueType = "integer"
	TypeFloat       ValueType = "number"
	TypeSelect      ValueType = "select"
	TypeFile        ValueType = "file"
	TypeText        ValueType = "textarea"
	TypeStringSlice ValueType = "string_slice"
	TypeObject      ValueType = "object"
)

type ReloadPolicy string

const (
	ReloadHot       ReloadPolicy = "hot"
	ReloadComponent ReloadPolicy = "component"
	ReloadProcess   ReloadPolicy = "process"
	ReloadBootOnly  ReloadPolicy = "boot_only"
)

type Validator func(value any) error

// Item describes one configuration value and contains the metadata required by
// validation and configuration GUIs.
type Item struct {
	Key          string         `json:"key"`
	Type         ValueType      `json:"type"`
	Default      any            `json:"default,omitempty"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description,omitempty"`
	Group        string         `json:"group,omitempty"`
	Options      []string       `json:"options,omitempty"`
	Accept       string         `json:"accept,omitempty"`
	Required     bool           `json:"required,omitempty"`
	Sensitive    bool           `json:"sensitive,omitempty"`
	Readonly     bool           `json:"readonly,omitempty"`
	Deprecated   bool           `json:"deprecated,omitempty"`
	Reload       ReloadPolicy   `json:"reloadPolicy,omitempty"`
	Min          *float64       `json:"min,omitempty"`
	Max          *float64       `json:"max,omitempty"`
	UI           map[string]any `json:"ui,omitempty"`
	ValidateFunc Validator      `json:"-"`
}

func (i Item) normalized() (Item, error) {
	i.Key = normalize_key(i.Key)
	if i.Key == "" {
		return Item{}, errors.New("configapi: schema item key is empty")
	}
	if i.Type == "" {
		i.Type = infer_value_type(i.Default)
	}
	if i.Reload == "" {
		i.Reload = ReloadProcess
	}
	if !valid_value_type(i.Type) {
		return Item{}, fmt.Errorf("configapi: unsupported value type %q for %s", i.Type, i.Key)
	}
	if !valid_reload_policy(i.Reload) {
		return Item{}, fmt.Errorf("configapi: unsupported reload policy %q for %s", i.Reload, i.Key)
	}
	if i.Type == TypeSelect && len(i.Options) == 0 {
		return Item{}, fmt.Errorf("configapi: select item %s has no options", i.Key)
	}
	if i.Min != nil && i.Max != nil && *i.Min > *i.Max {
		return Item{}, fmt.Errorf("configapi: minimum exceeds maximum for %s", i.Key)
	}
	if i.Default != nil {
		coerced, err := coerce_value(i.Type, i.Default)
		if err != nil {
			return Item{}, fmt.Errorf("configapi: invalid default for %s: %w", i.Key, err)
		}
		if err := i.validate(coerced); err != nil {
			return Item{}, fmt.Errorf("configapi: invalid default for %s: %w", i.Key, err)
		}
		i.Default = coerced
	}
	i.Options = append([]string(nil), i.Options...)
	if i.UI != nil {
		ui, err := clone_values(i.UI)
		if err != nil {
			return Item{}, fmt.Errorf("configapi: clone UI metadata for %s: %w", i.Key, err)
		}
		i.UI = ui
	}
	return i, nil
}

func (i Item) validate(value any) error {
	if value == nil {
		if i.Required {
			return errors.New("value is required")
		}
		return nil
	}
	if i.Type == TypeSelect {
		selected := fmt.Sprint(value)
		valid := false
		for _, option := range i.Options {
			if option == selected {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("must be one of %s", strings.Join(i.Options, ", "))
		}
	}
	if i.Min != nil || i.Max != nil {
		number, ok := numeric_value(value)
		if !ok {
			return errors.New("value is not numeric")
		}
		if i.Min != nil && number < *i.Min {
			return fmt.Errorf("must be greater than or equal to %v", *i.Min)
		}
		if i.Max != nil && number > *i.Max {
			return fmt.Errorf("must be less than or equal to %v", *i.Max)
		}
	}
	if i.ValidateFunc != nil {
		return i.ValidateFunc(value)
	}
	return nil
}

type ModuleDeclaration struct {
	Name  string `json:"name"`
	Items []Item `json:"items"`
}

// ModuleHost is a configuration provider that accepts module-owned schemas and
// can rebuild its effective configuration after a module is attached.
type ModuleHost interface {
	Provider
	RegisterModule(declaration ModuleDeclaration) (*ModuleHandle, error)
	Refresh(ctx context.Context) (UpdateResult, error)
}

func DeclareModule(name string, items ...Item) ModuleDeclaration {
	return ModuleDeclaration{Name: strings.TrimSpace(name), Items: append([]Item(nil), items...)}
}

// ModuleHandle unregisters one module's schema. Configuration source values
// are intentionally preserved so a hot-plugged module can recover them later.
type ModuleHandle struct {
	once       sync.Once
	unregister func()
}

func (h *ModuleHandle) Unregister() {
	if h == nil {
		return
	}
	h.once.Do(func() {
		if h.unregister != nil {
			h.unregister()
		}
	})
}

func schema_items_sorted(items map[string]Item) []Item {
	result := make([]Item, 0, len(items))
	for _, item := range items {
		copy_item := item
		copy_item.Options = append([]string(nil), item.Options...)
		if item.UI != nil {
			copy_item.UI, _ = clone_values(item.UI)
		}
		result = append(result, copy_item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func valid_value_type(value_type ValueType) bool {
	switch value_type {
	case TypeString, TypeBool, TypeInt, TypeFloat, TypeSelect, TypeFile, TypeText, TypeStringSlice, TypeObject:
		return true
	default:
		return false
	}
}

func valid_reload_policy(policy ReloadPolicy) bool {
	switch policy {
	case ReloadHot, ReloadComponent, ReloadProcess, ReloadBootOnly:
		return true
	default:
		return false
	}
}
