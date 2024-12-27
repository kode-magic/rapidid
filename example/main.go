package main

import (
	"fmt"
	"github.com/kode-magic/rapidid"
	"sort"
)

func main() {
	exampleOne()
	exampleTwo()
}

func exampleOne() {
	idStr0 := rapidid.Generate()
	idStr1 := rapidid.GenerateWithPrefix("rid")

	id2, err := rapidid.Parse(idStr1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("ID0 = %s\n", idStr0)
	fmt.Printf("ID1 = %s\n", idStr1) // prefixed with 'acc-'
	fmt.Printf("ID2 = %s\n", id2)    // prefixed with 'acc-'
}

func exampleTwo() {
	var ids []string
	var idsToBeSorted []string
	for i := 0; i < 100; i++ {
		idString := rapidid.GenerateWithPrefix("acc")
		id, err := rapidid.Parse(idString)
		if err != nil {
			panic(err)
		}
		ids = append(ids, id.String())
		idsToBeSorted = append(idsToBeSorted, id.String())

	}
	sort.Strings(idsToBeSorted) // sort one of the slice
	for i, sortedID := range idsToBeSorted {
		id := ids[i]
		if id != sortedID {
			panic(fmt.Sprintf("%s != %s\n", id, sortedID))
		}
		fmt.Printf("%s == %s\n", id, sortedID)
	}
}
