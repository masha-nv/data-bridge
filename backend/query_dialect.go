package main

import (
	"strconv"
	"strings"
)

func rebindQueryForDatabase(environment, databaseName, query string) (string, error) {
	definition, err := getDatabaseDefinition(environment, databaseName)
	if err != nil {
		return "", err
	}

	if definition.DBDriver != "postgres" {
		return query, nil
	}

	var builder strings.Builder
	parameterIndex := 1
	for _, character := range query {
		if character == '?' {
			builder.WriteString("$")
			builder.WriteString(strconv.Itoa(parameterIndex))
			parameterIndex++
			continue
		}

		builder.WriteRune(character)
	}

	return builder.String(), nil
}