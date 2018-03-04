package args

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
)

type ArgProgram int8

const (
	ArgProgramSendmail   ArgProgram = 1
	ArgProgramNewaliases ArgProgram = 2
	ArgProgramMailq      ArgProgram = 3
)

func Parse(argv []string) (ArgProgram, *SmArgs, *MqArgs, []string, error) {

	// prepare result variables
	resultProgram := ArgProgramSendmail
	var resultSmArgs *SmArgs
	var resultMqArgs *MqArgs
	var resultValues []string

	// detect program
	switch filepath.Base(argv[0]) {
	case "newaliases":
		resultProgram = ArgProgramNewaliases
	case "mailq":
		resultProgram = ArgProgramMailq
	}

	// prepare configuration
	config := []argConf{}

	switch resultProgram {
	case ArgProgramSendmail:
		{

			resultSmArgs = new(SmArgs)
			config = prepareConfig(
				reflect.TypeOf(*resultSmArgs),
				reflect.ValueOf(resultSmArgs).Elem())
		}
	case ArgProgramMailq:
		{
			resultMqArgs = new(MqArgs)
			config = prepareConfig(
				reflect.TypeOf(*resultMqArgs),
				reflect.ValueOf(resultMqArgs).Elem())
		}
	}

	// parse arguments
	assignValue := func(ci int, source string) error {
		err := convert(source, config[ci].value)
		if err != nil {
			return errors.New("Error while parsing " + config[ci].name + ": " + err.Error())
		}
		return nil
	}

	for ai := 1; ai < len(argv); ai++ {
		if strings.HasPrefix(argv[ai], "-") {
			// handle params
			handled := false
			for ci := 0; ci < len(config); ci++ {
				switch config[ci]._type {
				case argTypeFlag:
					if config[ci].name == argv[ai] {
						source := ""
						err := assignValue(ci, source)
						if err != nil {
							return resultProgram,
								resultSmArgs,
								resultMqArgs,
								resultValues,
								err
						}
						handled = true
						break
					}
				case argTypeTrailing:
					if config[ci].name == argv[ai] {
						if ai+1 >= len(argv) {
							return resultProgram,
								resultSmArgs,
								resultMqArgs,
								resultValues,
								errors.New("Value missing for argument " + argv[ai])
						}
						source := argv[ai+1]
						err := assignValue(ci, source)
						if err != nil {
							return resultProgram,
								resultSmArgs,
								resultMqArgs,
								resultValues,
								err
						}
						handled = true
						ai++
						break
					}
				case argTypeAttached:
					if strings.HasPrefix(argv[ai], config[ci].name) {
						source := argv[ai][len(config[ci].name):]
						err := assignValue(ci, source)
						if err != nil {
							return resultProgram,
								resultSmArgs,
								resultMqArgs,
								resultValues,
								err
						}
						handled = true
						break
					}
				}
			}
			// verify if argument had been processed
			if !handled {
				return resultProgram,
					resultSmArgs,
					resultMqArgs,
					resultValues,
					errors.New("Unknown argument " + argv[ai])
			}
		} else {
			// handle values
			resultValues = append(resultValues, argv[ai])
		}
	}

	// done
	return resultProgram,
		resultSmArgs,
		resultMqArgs,
		resultValues,
		nil
}