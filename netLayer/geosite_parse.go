package netLayer

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//来自 v2fly, 改动改了一下命名。

// GeositeRawList 用于序列化
type GeositeRawList struct {
	Name    string
	Domains []GeositeDomain
}

func LoadGeositeFile(path string) (*GeositeRawList, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	list := &GeositeRawList{
		Name: strings.ToUpper(filepath.Base(path)),
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = removeGeositeComment(line)
		if len(line) == 0 {
			continue
		}
		entry, err := parseGeoSiteEntry(line)
		if err != nil {
			return nil, err
		}
		list.Domains = append(list.Domains, entry)
	}

	return list, nil
}

func parseGeoSiteEntry(line string) (GeositeDomain, error) {
	line = strings.TrimSpace(line)
	parts := strings.Split(line, " ")

	var entry GeositeDomain
	if len(parts) == 0 {
		return entry, errors.New("empty entry")
	}

	if err := parseGeositeDomain(parts[0], &entry); err != nil {
		return entry, err
	}

	for i := 1; i < len(parts); i++ {
		attr, err := parseGeositeAttribute(parts[i])
		if err != nil {
			return entry, err
		}
		entry.Attrs = append(entry.Attrs, attr)
	}

	return entry, nil
}

func parseGeositeAttribute(attr string) (attribute GeositeAttr, err error) {
	if len(attr) == 0 || attr[0] != '@' {
		err = errors.New("invalid attribute: " + attr)
		return
	}

	attr = attr[1:]
	parts := strings.Split(attr, "=")
	if len(parts) == 1 {
		attribute.Key = strings.ToLower(parts[0])
		attribute.Value = true
	} else {
		attribute.Key = strings.ToLower(parts[0])
		var intv int
		intv, err = strconv.Atoi(parts[1])
		if err != nil {
			err = errors.New("invalid attribute: " + attr + ": " + err.Error())
			return
		}
		attribute.Value = intv
	}
	return
}

func parseGeositeDomain(domain string, entry *GeositeDomain) error {
	kv := strings.Split(domain, ":")
	if len(kv) == 1 {
		entry.Type = "domain"
		entry.Value = strings.ToLower(kv[0])
		return nil
	}

	if len(kv) == 2 {
		entry.Type = strings.ToLower(kv[0])
		entry.Value = strings.ToLower(kv[1])
		return nil
	}

	return errors.New("Invalid format: " + domain)
}

func removeGeositeComment(line string) string {
	idx := strings.Index(line, "#")
	if idx == -1 {
		return line
	}
	return strings.TrimSpace(line[:idx])
}

func isMatchGeositeAttr(Attrs []GeositeAttr, includeKey string) bool {
	isMatch := false
	mustMatch := true
	matchName := includeKey
	if strings.HasPrefix(includeKey, "!") {
		isMatch = true
		mustMatch = false
		matchName = strings.TrimLeft(includeKey, "!")
	}

	for _, Attr := range Attrs {
		attrName := Attr.Key
		if mustMatch {
			if matchName == attrName {
				isMatch = true
				break
			}
		} else {
			if matchName == attrName {
				isMatch = false
				break
			}
		}
	}
	return isMatch
}

func createGeositeIncludeAttrEntrys(list *GeositeRawList, matchAttr GeositeAttr) []GeositeDomain {
	newEntryList := make([]GeositeDomain, 0, len(list.Domains))
	matchName := matchAttr.Key
	for _, entry := range list.Domains {
		matched := isMatchGeositeAttr(entry.Attrs, matchName)
		if matched {
			newEntryList = append(newEntryList, entry)
		}
	}
	return newEntryList
}

//这里从v2fly改动了一些. 因为我发现 Inclusion最终不会被用到
func ParseGeositeList(list *GeositeRawList, ref map[string]*GeositeRawList) (*GeositeRawList, error) {
	inclu := make(map[string]bool)
	pl := &GeositeRawList{
		Name: list.Name,
		//Inclusion: make(map[string]bool),
	}
	entryList := list.Domains
	for {
		newEntryList := make([]GeositeDomain, 0, len(entryList))
		hasInclude := false
		for _, entry := range entryList {
			if entry.Type == "include" {
				refName := strings.ToUpper(entry.Value)
				if entry.Attrs != nil {
					for _, attr := range entry.Attrs {
						InclusionName := strings.ToUpper(refName + "@" + attr.Key)
						if inclu[InclusionName] {
							continue
						}
						inclu[InclusionName] = true

						refList := ref[refName]
						if refList == nil {
							return nil, errors.New(entry.Value + " not found.")
						}
						attrEntrys := createGeositeIncludeAttrEntrys(refList, attr)
						if len(attrEntrys) != 0 {
							newEntryList = append(newEntryList, attrEntrys...)
						}
					}
				} else {
					InclusionName := refName
					if inclu[InclusionName] {
						continue
					}
					inclu[InclusionName] = true
					refList := ref[refName]
					if refList == nil {
						return nil, errors.New(entry.Value + " not found.")
					}
					newEntryList = append(newEntryList, refList.Domains...)
				}
				hasInclude = true
			} else {
				newEntryList = append(newEntryList, entry)
			}
		}
		entryList = newEntryList
		if !hasInclude {
			break
		}
	}
	pl.Domains = entryList

	return pl, nil
}

func (grl *GeositeRawList) ToGeositeList() (gl *GeositeList) {
	gl = new(GeositeList)
	gl.Name = grl.Name
	gl.Domains = make(map[string]GeositeDomain)
	gl.FullDomains = make(map[string]GeositeDomain)
	gl.RegexDomains = make([]GeositeDomain, 0)
	for _, v := range grl.Domains {
		switch v.Type {
		case "domain":
			gl.Domains[v.Value] = v
		case "regexp":
			gl.RegexDomains = append(gl.RegexDomains, v)
		case "full":
			gl.FullDomains[v.Value] = v
		}
	}
	return
}
