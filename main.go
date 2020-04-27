package main

import (
	"bytes"
	"regexp"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"sort"
	"strings"
)

// ./jsonsorter -k "m.ary.id:m.ary.v.key;.*\.id$:.*.v$" -f tmp.json

type node struct {
	childs map[string]*node
}

func (n *node) getChild(child string) *node {
	return n.childs[child]
}

var root = &node{
	childs: make(map[string]*node),
}

var regex []string

func parseKeys2Node(keys string) {
	strs := strings.Split(keys, ";")
	keydotkeyStr := strs[0]
	regexs := strs[1]

	keyDotKeys := strings.Split(keydotkeyStr, ":")
	for _, key := range keyDotKeys {
		fields := strings.Split(key, ".")
		now := root
		for _, field := range fields {
			if child, exist := now.childs[field]; exist {
				now = child
			} else {
				child := &node{
					childs: make(map[string]*node),
				}
				now.childs[field] = child
				now = child
			}
		}
	}

	r := strings.Split(regexs, ":")
	for _, reg := range r {
		if reg != "" {
			regex = append(regex, reg)
		}
	}
}

func main() {
	fileName := flag.String("f", "", "the lua file name")
	keys := flag.String("k", ";^.*\\.id$", "keys to be sorted")
	flag.Parse()
	if *fileName == "" {
		log.Fatal("filename is empty")
	}
	parseKeys2Node(*keys)

	content, err := ioutil.ReadFile(*fileName)
	if err != nil {
		log.Fatal("read file fail", err)
	}

	var d map[string]interface{}
	if err := json.Unmarshal(content, &d); err != nil {
		log.Fatal("unmarshal fail", err)
	}

	buf.WriteString("{\n")
	outputMap(d, 0, root, "")
	buf.WriteString("}\n")

	fmt.Println(buf.String())
}

var buf = &bytes.Buffer{}

func getMapKeys(m map[string]interface{}) (res []string) {
	for k, _ := range m {
		res = append(res, k)
	}

	sort.Sort(sort.StringSlice(res))
	return
}

type UniqueAry struct {
	field string
	a     []interface{}
}

func (p UniqueAry) Len() int { return len(p.a) }
func (p UniqueAry) Less(i, j int) bool {
	fields := strings.Split(p.field, "|")

	for _, field := range fields {
		if field == "" {
			continue
		}

		ma := p.a[i].(map[string]interface{})
		mb := p.a[j].(map[string]interface{})
		_, exista := ma[field];
		_, existb := mb[field]
		if !exista && !existb {
			continue
		}
		if !exista {
			return true
		}
		if !existb {
			return false
		}

		switch ma[field].(type) {
		case float64:
			if ma[field].(float64) == mb[field].(float64) {
				continue
			}
			return ma[field].(float64) < mb[field].(float64)
		case string:
			if ma[field].(string) == mb[field].(string) {
				continue
			}
			return ma[field].(string) < mb[field].(string)
		}
	}
	return true
}

func (p UniqueAry) Swap(i, j int) { p.a[i], p.a[j] = p.a[j], p.a[i] }

func outputMap(m map[string]interface{}, depth int, now *node, joinedKeys string) error {
	var keys = getMapKeys(m)
	for _, k := range keys {
		buf.WriteString(fmt.Sprintf(`%v"%v": `, tabWithDepth(depth+1), k))

		var tmpJoinedKey string
		if len(joinedKeys) > 0 {
			tmpJoinedKey = joinedKeys + "." + k
		} else {
			tmpJoinedKey = k
		}
		var child *node
		if now != nil {
			if c := now.getChild(k); c != nil {
				child = c
			}
		}

		v := m[k]
		switch reflect.TypeOf(v).Kind() {
		case reflect.Float64, reflect.Bool:
			buf.WriteString(fmt.Sprintf("%v", v))
		case reflect.String:
			buf.WriteString(fmt.Sprintf(`"%v"`, v))
		case reflect.Map:
			buf.WriteString("{\n")
			outputMap(v.(map[string]interface{}), depth+1, child, tmpJoinedKey)
			buf.WriteString(fmt.Sprintf("%v}", tabWithDepth(depth+1)))
		case reflect.Array, reflect.Slice:
			buf.WriteString(fmt.Sprintf("[\n"))
			ary := v.([]interface{})
			if len(ary) > 0 {
				switch reflect.TypeOf(ary[0]).Kind() {
				case reflect.Float64, reflect.Bool:
					for _, aryEle := range ary {
						buf.WriteString(fmt.Sprintf("%v%v,\n", tabWithDepth(depth+2), aryEle))
					}
					buf = bytes.NewBuffer(buf.Bytes()[:buf.Len()-2])
				case reflect.String:
					for _, aryEle := range ary {
						buf.WriteString(fmt.Sprintf("%v\"%v\",\n", tabWithDepth(depth+2), aryEle))
					}
					buf = bytes.NewBuffer(buf.Bytes()[:buf.Len()-2])
				case reflect.Map:
					ele0 := ary[0].(map[string]interface{})

					// by connected key, a.b.c
					if child != nil {
						for field, cc := range child.childs {
							if len(cc.childs) > 0 { // 不是终点，不排序
								continue
							}

							sort.Sort(&UniqueAry{
								field: field,
								a: ary,
							})
						}
					}

					// by regex .*.id$
					for field, _ := range ele0 {
						ttJK := tmpJoinedKey + "." + field
						for _, reg := range regex {
							r := regexp.MustCompile(reg)
							if r.Match([]byte(ttJK)) {

								sort.Sort(&UniqueAry{
									field: field,
									a: ary,
								})
							}
						}
					}

					for _, aryEle := range ary {
						buf.WriteString(fmt.Sprintf("%v{\n", tabWithDepth(depth+2)))
						outputMap(aryEle.(map[string]interface{}), depth+2, child, tmpJoinedKey)
						buf.WriteString(fmt.Sprintf("%v},\n", tabWithDepth(depth+2)))
					}
					buf = bytes.NewBuffer(buf.Bytes()[:buf.Len()-2])
				}
			}
			buf.WriteString("\n")
			buf.WriteString(fmt.Sprintf("%v]", tabWithDepth(depth+1)))
		default:
			buf.WriteString("unknown type" + reflect.TypeOf(v).Kind().String() + "\n")
		}
		buf.WriteString(",\n")
	}
	if len(keys) > 0 {
		buf = bytes.NewBuffer(buf.Bytes()[:buf.Len()-2])
		buf.WriteString("\n")
	}
	return nil
}

func tabWithDepth(depth int) string {
	res := ""
	for i := 0; i < depth; i++ {
		res += "\t"
	}
	return res
}
