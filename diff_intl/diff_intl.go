package main

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"

	"github.com/gopherjs/gopherjs/js"
)

func main() {
	js.Global.Set("unown", map[string]interface{}{
		"Diff": Diff,
	})
	/*
		if len(os.Args) != 3 {
			fmt.Printf("usage: %s new-intl-text-file old-intl-text-file\n", os.Args[0])
			os.Exit(-1)
		}
		var newFilePath = os.Args[1]
		var oldFilePath = os.Args[2]

		var oldString, newString string
		var err error

		if oldString, err = readFileString(oldFilePath); err != nil {
			fmt.Printf("unable to read %s: %s", oldFilePath, err)
		}
		if newString, err = readFileString(newFilePath); err != nil {
			fmt.Printf("unable to read %s: %s", newFilePath, err)
		}
	*/

}

func Diff(oldString, newString string) (string, string, error) {

	var err error

	var oldMapSections, newMapSections []Section
	var oldNormalSections, newNormalSections []Section

	oldMapSections, oldNormalSections, err = parse(oldString)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse the older intl file: %s", err)
	}

	newMapSections, newNormalSections, err = parse(newString)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse the newer intl file: %s", err)
	}

	var normalSummary, mapSummary string

	if normalSummary, err = diffSections(oldNormalSections, newNormalSections, false); err != nil {
		return "", "", fmt.Errorf("failed to check diff:%s", err)
	}
	if mapSummary, err = diffSections(oldMapSections, newMapSections, true); err != nil {
		return "", "", fmt.Errorf("failed to check diff:%s", err)
	}

	//fmt.Println(oldMapSections) // DEBUG
	/*
		fmt.Println("================Regular Sections================")
		//pass(normalSummary)
		fmt.Println(normalSummary)
		fmt.Println("================Map sections================")
		fmt.Println(mapSummary)
	*/

	return normalSummary, mapSummary, nil
}

//func pass(a ...interface{}) {}

func diffSections(oldSections []Section, newSections []Section, isMap bool) (string, error) {
	var strMap string
	if isMap {
		strMap = "Map"
	} else {
		strMap = ""
	}

	var oldSectionsHashmap = make(map[int]Section)
	for _, section := range oldSections {
		if _, isExist := oldSectionsHashmap[section.ID]; isExist {
			fmt.Printf("//warn: duplicated section in the old intl file: %s%d\n", strMap, section.ID)
		}
		oldSectionsHashmap[section.ID] = section
	}

	var newSectionsSet = make(map[int]struct{})

	var result_addedSections string
	var result_removedSections string
	var result_changedSections string

	for _, newSection := range newSections {
		if _, isExist := newSectionsSet[newSection.ID]; isExist {
			fmt.Printf("//warn: duplicated section in the old intl file: %s%d\n", strMap, newSection.ID)

		}
		newSectionsSet[newSection.ID] = struct{}{}

		if oldSection, isExist := oldSectionsHashmap[newSection.ID]; isExist {
			/*
				if oldSection.Type != newSection.Type {
					return "", fmt.Errorf("mismatched section type: ")
				}
			*/
			var sectionResult string
			var err error
			if oldSection.Type == ArraySection {
				sectionResult, err = diffArraySections(oldSection, newSection)
			} else {
				sectionResult, err = diffDictSections(oldSection, newSection)
				//fmt.Println(newSection.ID, len(newSection.Data.([]Text))) // DEBUG
			}

			if err != nil {
				return "", err
			}

			if sectionResult != "" {
				if result_changedSections == "" {
					result_changedSections = "//FORMAT: (+/-/Δ|LINE NUMBER IN FILE|LINE NUMBER IN SECTION):TEXT" //"//format: (+(added)/-(removed)/Δ(changed(only used in array sections))|line no. in the whole file|line no. in its section): text\n\n\n"
				}
				result_changedSections += "[" + strMap + strconv.Itoa(newSection.ID) + "]\n" + sectionResult + "\n\n"
			}

			delete(oldSectionsHashmap, newSection.ID)
		} else {
			if result_addedSections == "" {
				result_addedSections = "ADDED SECTIONS:\n"
			}
			result_addedSections += strMap + strconv.Itoa(newSection.ID) + "\n"
		}

	}

	if len(oldSectionsHashmap) > 0 {
		var removedSections = make([]int, len(oldSectionsHashmap))
		var i = 0
		for key := range oldSectionsHashmap {
			removedSections[i] = key
			i++
		}
		sort.Ints(removedSections)
		result_removedSections = "REMOVED SECTIONS:\n"
		for _, sectionID := range removedSections {
			result_removedSections += strMap + strconv.Itoa(sectionID) + "\n"
		}
	}

	var results = []string{}
	if result_removedSections != "" {
		results = append(results, result_removedSections)
	}
	if result_addedSections != "" {
		results = append(results, result_addedSections)
	}
	if result_changedSections != "" {
		results = append(results, result_changedSections)
	}

	var summary string

	for i, result := range results {
		summary += result
		if i+1 < len(results) {
			summary += "\n\n\n\n"
		}
	}

	return summary, nil

}

type ArraySortor []IndexTextPair

func (a ArraySortor) Less(i, j int) bool { return a[i].Index < a[j].Index }
func (a ArraySortor) Len() int           { return len(a) }
func (a ArraySortor) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// diffArraySections checks the diff between two sections. You should notice that the order of section data might be changed
func diffArraySections(oldSection, newSection Section) (string, error) {
	var oldArray, newArray []IndexTextPair
	/*
		var ok bool
		if oldArray, ok = oldSection.Data.([]IndexTextPair); !ok {
		}
	*/
	oldArray = oldSection.Data.([]IndexTextPair)
	newArray = newSection.Data.([]IndexTextPair)
	sort.Sort(ArraySortor(oldArray))
	sort.Sort(ArraySortor(newArray))

	var result string

	var i_old = 0
	for _, pair := range newArray {
		if i_old < len(oldArray) {
			var oldPair = oldArray[i_old]
			if oldPair.Index > pair.Index { // 新的多了
				result += fmt.Sprintf("(+|%d|%d):%d:%s\n", pair.LineNo, pair.SectionLineNo, pair.Index, pair.Text)
			} else if oldPair.Index < pair.Index { //新的少了 // 懒得调整代码结构了
				for i_old++; oldArray[i_old].Index < pair.Index; i_old++ {
					oldPair = oldArray[i_old]
					result += fmt.Sprintf("(-|%d|%d):%d:%s\n", oldPair.LineNo, oldPair.SectionLineNo, oldPair.Index, oldPair.Text)
				}
			} else {
				if pair.Text != oldPair.Text {
					var a, b string
					if oldPair.LineNo != pair.LineNo {
						a = fmt.Sprintf("%d→%d", oldPair.LineNo, pair.LineNo)
					} else {
						a = strconv.Itoa(oldPair.LineNo)
					}
					if oldPair.SectionLineNo != pair.SectionLineNo {
						b = fmt.Sprintf("%d→%d", oldPair.SectionLineNo, pair.SectionLineNo)
					} else {
						b = strconv.Itoa(oldPair.SectionLineNo)
					}

					result += fmt.Sprintf("(Δ|%s|%s):%d:%s→%s\n", a, b, oldPair.Index, oldPair.Text, pair.Text)
				}
				i_old++
			}
		}
	}

	return result, nil
}

// diffDictSections ...
func diffDictSections(oldSection, newSection Section) (string, error) {

	var oldHashmap = make(map[string]Text)

	var oldDict = oldSection.Data.([]Text)
	var newDict = newSection.Data.([]Text)

	var result_added, result_removed string

	for _, text := range oldDict {
		oldHashmap[text.Text] = text
	}

	for _, text := range newDict {
		if _, isExisted := oldHashmap[text.Text]; isExisted {
			delete(oldHashmap, text.Text)
		} else {
			result_added += fmt.Sprintf("(+|%d|%d):%s\n", text.LineNo, text.SectionLineNo, text.Text)
		}
	}

	var removedTexts = make([]Text, len(oldHashmap))

	var i = 0
	for _, text := range oldHashmap {
		removedTexts[i] = text
		i++
	}
	sort.Sort(TextArraySortor(removedTexts))

	for _, text := range removedTexts {
		result_removed += fmt.Sprintf("(-|%d|%d):%s\n", text.LineNo, text.SectionLineNo, text.Text)
	}

	if result_added != "" && result_removed != "" {
		return result_removed + "\n" + result_added, nil
	}
	return result_removed + result_added, nil
}

type TextArraySortor []Text

func (tas TextArraySortor) Less(i, j int) bool { return tas[i].LineNo < tas[j].LineNo }
func (tas TextArraySortor) Len() int           { return len(tas) }
func (tas TextArraySortor) Swap(i, j int)      { tas[i], tas[j] = tas[j], tas[i] }

// readFileString gets rid of BOM and replace CRLF to LF
func readFileString(fileName string) (string, error) {
	var fileData []byte
	var err error
	if fileData, err = ioutil.ReadFile(fileName); err != nil {
		return "", err
	}
	if len(fileData) < 3 {
		return string(fileData), nil
	}
	if fileData[0] == 0xef && fileData[1] == 0xbb && fileData[2] == 0xbf {
		fileData = fileData[3:]
	}
	var fileString = string(fileData)
	fileString = strings.Replace(fileString, "\r\n", "\n", -1)
	fileString = strings.Replace(fileString, "\r", "\n", -1)
	return fileString, nil
}

type IndexTextPair struct {
	LineNo        int // 第一行的
	SectionLineNo int

	Index int
	Text  string
}

type Text struct {
	LineNo        int
	SectionLineNo int

	Text string
}

const (
	ArraySection int = iota
	DictSection
)

type Section struct {
	Type int
	ID   int
	Data interface{} // []IndexTextPair || []Text
}

//
func parse(str string) ([]Section, []Section, error) {
	var lines = strings.Split(str, "\n")

	var _isArray = [...]bool{false /*placeholder for 0*/, true, true, true, true, true, true, true, true, true, true,
		true, true, false, false /*14: ?*/, false /*?*/, false /*?*/, true, false, false, true, false, false}

	var normalSections = []Section{}
	var mapSections = []Section{}

	/*
		1: this line should be a array index ( or section label )
		2: this line should be the original text ( or section label )
		3: this line should be the translated text
	*/
	var state = -1

	var sectionID = -1
	var isArray bool
	var isMapSection bool

	var array []IndexTextPair
	var dict []Text

	var sectionLineNo int
	var arrayIndex int

	var err error

	for lineNo, line := range lines {
		sectionLineNo++
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		//
		if state == -1 || state == 1 || state == 2 {
			if line[0] == '[' {
				line = strings.TrimSpace(line)
				if len(line) == 1 {
					return nil, nil, fmt.Errorf("line %d: unable to parse section label:\n%s", lineNo+1, line)
				}

				if state != -1 {
					//fmt.Println(sectionID, isMapSection) // DEBUG
					if isMapSection {
						mapSections = append(mapSections, Section{DictSection, sectionID, dict})
					} else {
						if isArray {
							normalSections = append(normalSections, Section{ArraySection, sectionID, array})
						} else {
							normalSections = append(normalSections, Section{DictSection, sectionID, dict})
						}
					}
				}

				if len(line) > 5 && strings.ToLower(line[1:4]) == "map" {
					isMapSection = true
					sectionID, err = strconv.Atoi(line[4 : len(line)-1])
				} else {
					isMapSection = false
					sectionID, err = strconv.Atoi(line[1 : len(line)-1])
				}

				if err != nil {
					return nil, nil, fmt.Errorf("line %d: unable to parse section label:\n%s", lineNo+1, line)
				}

				sectionLineNo = 1

				if isMapSection {
					isArray = false
					state = 2
					dict = make([]Text, 0)
				} else {
					if sectionID > len(_isArray) {
						return nil, nil, fmt.Errorf("line %d: section is out of range:\n%s", lineNo+1, line)
					}

					isArray = _isArray[sectionID]
					if isArray {
						state = 1
						array = make([]IndexTextPair, 0)
					} else {
						state = 2
						dict = make([]Text, 0)
					}
				}

				continue
			}
		}

		switch state {
		case 1: // array index
			arrayIndex, err = strconv.Atoi(strings.TrimSpace(line))
			if err != nil {
				return nil, nil, fmt.Errorf("line %d: unable to parse array index:\n%s", lineNo+1, line)
			}
			state = 2
		case 2: // original text
			if isArray {
				array = append(array, IndexTextPair{lineNo + 1, sectionLineNo, arrayIndex, line})
			} else {
				dict = append(dict, Text{lineNo + 1, sectionLineNo, line})
			}
			state = 3
		case 3: // translated line
			if isArray {
				state = 1
			} else {
				state = 2
			}
		}
	}

	if isMapSection {
		mapSections = append(mapSections, Section{DictSection, sectionID, dict})
	} else {
		if isArray {
			normalSections = append(normalSections, Section{ArraySection, sectionID, array})
		} else {
			normalSections = append(normalSections, Section{DictSection, sectionID, dict})
		}
	}

	return mapSections, normalSections, nil
}
