package proto

import (
	"regexp"
	"strconv"
	"strings"
)

// ProtoFile holds the parsed contents of a .proto file.
type ProtoFile struct {
	Package  string
	Services []ServiceDef
	Messages []MessageDef
}

// ServiceDef represents a proto service block.
type ServiceDef struct {
	Name string
	RPCs []RPCDef
}

// RPCDef represents a single RPC method.
type RPCDef struct {
	Name         string
	RequestType  string
	ResponseType string
}

// MessageDef represents a proto message.
type MessageDef struct {
	Name   string
	Fields []FieldDef
}

// FieldDef represents a single field inside a message.
type FieldDef struct {
	Name     string
	Type     string
	Number   int
	Repeated bool
}

var (
	rePackage = regexp.MustCompile(`^package\s+([\w.]+)\s*;`)
	reService = regexp.MustCompile(`^service\s+(\w+)\s*\{`)
	reRPC     = regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s+returns\s*\(\s*(\w+)\s*\)`)
	reMessage = regexp.MustCompile(`^message\s+(\w+)\s*\{`)
	reField   = regexp.MustCompile(`^(repeated\s+|optional\s+)?(\w+)\s+(\w+)\s*=\s*(\d+)\s*;`)
)

// ParseProto parses .proto file text and returns a ProtoFile.
func ParseProto(content string) (*ProtoFile, error) {
	pf := &ProtoFile{}

	type frame struct {
		kind string // "service" | "message"
		idx  int    // index into pf.Services or pf.Messages
	}

	var stack []frame
	depth := 0

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)

		// strip line comments
		if ci := strings.Index(line, "//"); ci >= 0 {
			line = strings.TrimSpace(line[:ci])
		}
		if line == "" {
			continue
		}

		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")

		// ── top-level declarations ────────────────────────────────────────
		if depth == 0 {
			if m := rePackage.FindStringSubmatch(line); m != nil {
				pf.Package = m[1]
			}
		}

		if m := reService.FindStringSubmatch(line); m != nil && depth == 0 {
			pf.Services = append(pf.Services, ServiceDef{Name: m[1]})
			stack = append(stack, frame{"service", len(pf.Services) - 1})
		} else if m := reMessage.FindStringSubmatch(line); m != nil && depth == 0 {
			pf.Messages = append(pf.Messages, MessageDef{Name: m[1]})
			stack = append(stack, frame{"message", len(pf.Messages) - 1})
		}

		depth += opens

		// ── inside a service ──────────────────────────────────────────────
		if depth == 1 && len(stack) > 0 && stack[len(stack)-1].kind == "service" {
			if m := reRPC.FindStringSubmatch(line); m != nil {
				svc := &pf.Services[stack[len(stack)-1].idx]
				svc.RPCs = append(svc.RPCs, RPCDef{
					Name:         m[1],
					RequestType:  m[2],
					ResponseType: m[3],
				})
			}
		}

		// ── inside a message (depth==1) ───────────────────────────────────
		if depth == 1 && len(stack) > 0 && stack[len(stack)-1].kind == "message" {
			if m := reField.FindStringSubmatch(line); m != nil {
				modifier := strings.TrimSpace(m[1])
				fieldType := m[2]
				fieldName := m[3]
				fieldNum, _ := strconv.Atoi(m[4])

				// skip proto keywords mistaken as type
				if fieldType == "optional" || fieldType == "repeated" {
					continue
				}

				msg := &pf.Messages[stack[len(stack)-1].idx]
				msg.Fields = append(msg.Fields, FieldDef{
					Name:     fieldName,
					Type:     fieldType,
					Number:   fieldNum,
					Repeated: modifier == "repeated",
				})
			}
		}

		depth -= closes
		if depth < 0 {
			depth = 0
		}

		// pop stack when we close a block
		if closes > 0 && depth == 0 && len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
	}

	return pf, nil
}
