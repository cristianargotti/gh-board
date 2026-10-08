package commands

import "github.com/cristianargotti/gh-board/internal/domain"

func (s *writeSession) writeNewFields(it domain.Item, options *writeNewOptions) error {
	fields := []struct {
		value string
		apply func(string) error
	}{
		{options.lane, func(v string) error { return s.writeMappedField(it, s.cfg.Capabilities.Lane, "lane", v) }},
		{options.epic, func(v string) error { return s.writeNewEpic(it, v) }},
		{options.sprint, func(v string) error { return s.writeSprint(it, v) }},
		{options.estimate, func(v string) error { return s.writeEstimate(it, v) }},
		{options.parent, func(v string) error { return s.writeLink(it, v) }},
	}
	for _, field := range fields {
		if field.value != "" {
			if err := field.apply(field.value); err != nil {
				return err
			}
		}
	}
	if options.start != "" || options.target != "" {
		return s.writeDates(it, options.start, options.target)
	}
	return nil
}

func (s *writeSession) writeNewEpic(it domain.Item, value string) error {
	if s.cfg.Capabilities.Epic == nil {
		return writeUnavailable(writeKindEpic)
	}
	return s.writeMappedField(it, &domain.FieldCapability{Field: s.cfg.Capabilities.Epic.Field}, writeKindEpic, value)
}
