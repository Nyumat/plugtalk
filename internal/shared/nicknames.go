package shared

import (
	"fmt"
	"math/rand"

	"plugtalk/data"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func GenerateNickname() string {
	adjective := data.Adjectives[rand.Intn(len(data.Adjectives))]
	animal := data.Animals[rand.Intn(len(data.Animals))]
	tc := cases.Title(language.English)
	adjective = tc.String(adjective)
	animal = tc.String(animal)
	return fmt.Sprintf("%s%s", adjective, animal)
}
