package cli

import (
	"fmt"
	"strings"
)

type commandLine struct {
	positionals []string

	// flags hold a true/false value
	flags map[string]bool

	// options carry a value
	options map[string][]string

	// counts accumulate repetition (e.g. -vvv)
	counts map[string]int
}

func parseCommandLine(args []string, boolFlags, countFlags []string) (parsed commandLine, err error) {
	parsed.flags = make(map[string]bool)
	parsed.options = make(map[string][]string)
	parsed.counts = make(map[string]int)

	// Create lookup tables for no-value flags. Boolean and count flags both take
	// no value; countSet distinguishes the two, noValueSet covers either.
	countSet := make(map[string]struct{}, len(countFlags))
	for _, name := range countFlags {
		countSet[name] = struct{}{}
	}
	noValueSet := make(map[string]struct{}, len(boolFlags)+len(countFlags))
	for _, name := range boolFlags {
		noValueSet[name] = struct{}{}
	}
	for _, name := range countFlags {
		noValueSet[name] = struct{}{}
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			// Stop parsing flags & options and treat every following arg as normal
			// positional arguments. Our parsing is done after this
			parsed.positionals = append(parsed.positionals, args[i+1:]...)
			break
		}

		isLong := len(arg) >= 3 && strings.HasPrefix(arg, "--")
		isShort := len(arg) >= 2 && arg[0] == '-' && arg[1] != '-'
		isPositional := !isLong && !isShort

		if isPositional {
			parsed.positionals = append(parsed.positionals, arg)
			continue
		}

		// Remove the prefix from the arg
		if isLong {
			arg = arg[2:]
		} else {
			arg = arg[1:]
		}

		// Special handling for short concatenated flags
		if isShort && len(arg) > 1 {
			allNoValue := true
			for j := 0; j < len(arg); j++ {
				name := string(arg[j])
				if _, isFlag := noValueSet[name]; !isFlag {
					allNoValue = false
					break
				}
			}

			if allNoValue {
				for j := 0; j < len(arg); j++ {
					name := string(arg[j])
					if _, isCount := countSet[name]; isCount {
						parsed.counts[name]++
					} else {
						parsed.flags[name] = true
					}
				}
				continue
			}
		}

		// Split the argument into a name -> value pair
		var name, value string
		var hasValue bool

		if left, right, found := strings.Cut(arg, "="); found {
			name = left
			value = right
			hasValue = true
		} else {
			name = arg
			hasValue = false
		}

		// Count flags take no value; each occurrence increments the tally.
		if _, isCount := countSet[name]; isCount {
			if hasValue {
				return parsed, fmt.Errorf("count flag %q does not take a value", name)
			}
			parsed.counts[name]++
			continue
		}

		// Check if the argument is a flag (boolean)
		if _, isFlag := noValueSet[name]; isFlag {
			parsedBool, ok := parseBool(value, hasValue)
			if !ok {
				return parsed, fmt.Errorf("flag %q expects true or false", name)
			}
			parsed.flags[name] = parsedBool
			continue
		}

		// If there wasnt an equal sign after the option name then the
		// value must be in the next argument
		if !hasValue && i+1 < len(args) && !isNamedInput(args[i+1]) {
			i++
			value = args[i]
			hasValue = true
		}

		if !hasValue {
			return parsed, fmt.Errorf("option %q expects a value", name)
		}

		parsed.options[name] = append(parsed.options[name], value)
	}

	return
}

func (parsed *commandLine) normalize(flags *flagSet) {
	if flags == nil {
		return
	}

	// Create a lookup table between short to long flags
	shortToLong := make(map[string]string, len(flags.fields))
	for _, flag := range flags.fields {
		if flag.Short == "" {
			continue
		}
		shortToLong[flag.Short] = flag.Name
	}

	for name, value := range parsed.flags {
		normalizedName, found := shortToLong[name]
		if found {
			delete(parsed.flags, name)
			parsed.flags[normalizedName] = value
		}
	}

	for name, values := range parsed.options {
		normalizedName, found := shortToLong[name]
		if found {
			delete(parsed.options, name)
			parsed.options[normalizedName] = append(parsed.options[normalizedName], values...)
		}
	}

	for name, value := range parsed.counts {
		normalizedName, found := shortToLong[name]
		if found {
			delete(parsed.counts, name)
			parsed.counts[normalizedName] += value
		}
	}
}

func (parsed *commandLine) validate(flags *flagSet, expectedPositionals []commandArgument) error {
	parsed.normalize(flags)

	// Create a lookup table for flag names
	flagLookup := make(map[string]flagField)
	if flags != nil {
		for _, flag := range flags.fields {
			flagLookup[flag.Name] = flag
		}
	}

	for name := range parsed.flags {
		flag, found := flagLookup[name]
		if !found {
			return fmt.Errorf("unknown flag %q", name)
		}
		if !flag.Bool {
			return fmt.Errorf("option %q used as boolean flag", name)
		}
	}

	for name := range parsed.options {
		flag, found := flagLookup[name]
		if !found {
			return fmt.Errorf("unknown option %q", name)
		}
		if flag.Bool {
			return fmt.Errorf("flag %q used as value option", name)
		}
	}

	for name := range parsed.counts {
		flag, found := flagLookup[name]
		if !found {
			return fmt.Errorf("unknown flag %q", name)
		}
		if !flag.Count {
			return fmt.Errorf("flag %q is not a count flag", name)
		}
	}

	if err := validatePositionals(parsed.positionals, expectedPositionals); err != nil {
		return err
	}

	return nil
}

func parseBool(s string, hasValue bool) (value, ok bool) {
	if !hasValue {
		return true, true
	}

	switch s {
	case "true":
		return true, true
	case "false":
		return false, true
	}

	return false, false
}

func isNamedInput(arg string) bool {
	return len(arg) >= 2 && arg[0] == '-'
}

func validatePositionals(provided []string, expected []commandArgument) error {
	required := 0
	maximum := len(expected)
	repeated := false

	for _, arg := range expected {
		switch arg.Kind {
		case requiredArgument:
			required++
		case repeatedArgument:
			repeated = true
			maximum = -1
		}
	}

	if len(provided) < required {
		return fmt.Errorf("missing arguments: got %d, want at least %d", len(provided), required)
	}
	if !repeated && len(provided) > maximum {
		return fmt.Errorf("too many arguments: got %d, want at most %d", len(provided), maximum)
	}

	return nil
}
