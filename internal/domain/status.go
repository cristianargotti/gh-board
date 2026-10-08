package domain

// BuiltinStatusField is the name GitHub gives the single select field every
// project is created with; section 5.4 calls it the one unambiguous field.
// Generic mode, without board.yml, reads it from discovery by this name.
const BuiltinStatusField = "Status"

// Generic reports whether the kit runs without board.yml: a nil
// configuration means every capability is unavailable.
func (c *Config) Generic() bool { return c == nil }

// CapabilitiesOf returns the declared capabilities, all nil in generic mode.
func CapabilitiesOf(cfg *Config) Capabilities {
	if cfg == nil {
		return Capabilities{}
	}
	return cfg.Capabilities
}

// StatusFieldName returns the configured status field name, or the
// built-in one in generic mode.
func StatusFieldName(cfg *Config) string {
	caps := CapabilitiesOf(cfg)
	if caps.Status != nil && caps.Status.Field != "" {
		return caps.Status.Field
	}
	return BuiltinStatusField
}

// StatusField finds the status single select field on the discovered
// project.
func StatusField(cfg *Config, project Project) (Field, bool) {
	f, ok := project.FieldByName(StatusFieldName(cfg))
	if !ok || f.DataType != DataTypeSingleSelect {
		return Field{}, false
	}
	return f, true
}

// ClassifyStatus classifies an option; generic mode knows no classes.
func ClassifyStatus(cfg *Config, option string) StatusClass {
	return CapabilitiesOf(cfg).Status.Classify(option)
}

// ItemStatus returns the status value of an item and its class.
func ItemStatus(cfg *Config, it Item) (string, StatusClass) {
	value := it.Text(StatusFieldName(cfg))
	return value, ClassifyStatus(cfg, value)
}

// IsActive reports whether the item sits in an active status.
func IsActive(cfg *Config, it Item) bool {
	_, class := ItemStatus(cfg, it)
	return class == StatusActive
}

// IsDone reports whether the item is finished: a done status when the board
// declares classes, otherwise a closed issue.
func IsDone(cfg *Config, it Item) bool {
	_, class := ItemStatus(cfg, it)
	if class != StatusUnknown {
		return class == StatusDone
	}
	return !it.IsOpen()
}

// Workable returns the open, unarchived items.
func Workable(items []Item) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.IsOpen() && !it.Archived {
			out = append(out, it)
		}
	}
	return out
}

// Pending returns the workable items that are not finished: the set the
// alert rules and the prompt sections read. An open issue parked in a done
// status is finished work waiting for the board workflow, not pending.
func Pending(cfg *Config, items []Item) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range Workable(items) {
		if !IsDone(cfg, it) {
			out = append(out, it)
		}
	}
	return out
}
