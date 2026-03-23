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
	Internal     bool       // true when marked with the Internal keyword
	HTTPMethod   HTTPMethod // explicit HTTP method (GET/POST/PUT/PATCH/DELETE)
	HTTPPath     string     // HTTP path template (Google API or Echo format)
	HTTPBody     string     // body field for POST/PUT/PATCH ("*" or field name)
}

// MessageDef represents a proto message.
type MessageDef struct {
	Name   string
	Fields []FieldDef
}

// FieldDef represents a single field inside a message.
type FieldDef struct {
	Name     string
	Type     string // may be qualified, e.g. "google.protobuf.FieldMask"
	Number   int
	Repeated bool
}

var (
	rePackage       = regexp.MustCompile(`^package\s+([\w.]+)\s*;`)
	reService       = regexp.MustCompile(`^service\s+(\w+)\s*\{`)
	reRPC           = regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s+returns\s*\(\s*([\w.]+)\s*\)(.*)`)
	reRoute         = regexp.MustCompile(`(GET|POST|PUT|DELETE|PATCH)\s+(\S+)`)
	reMessage       = regexp.MustCompile(`^message\s+(\w+)\s*\{`)
	reField         = regexp.MustCompile(`^(repeated\s+|optional\s+)?([\w.]+)\s+(\w+)\s*=\s*(\d+)\s*;`)
	reHTTPOption    = regexp.MustCompile(`option\s+\(google\.api\.http\)\s*=\s*\{`)
	reHTTPMethodOpt = regexp.MustCompile(`(get|post|put|patch|delete)\s*:\s*"([^"]+)"`)
	reBodyOpt       = regexp.MustCompile(`body\s*:\s*"([^"]+)"`)
)

// ParseProto parses .proto file text and returns a ProtoFile.
func ParseProto(content string) (*ProtoFile, error) {
	pf := &ProtoFile{}

	// frame tracks the current nesting context.
	type frame struct {
		kind      frameKind
		svcIdx    int // service index (used by service, rpc_options, http_option)
		msgIdx    int // message index (used by message)
		rpcIdx    int // RPC index within service (used by rpc_options, http_option)
		pushDepth int // depth at the START of the line that opened this block
	}

	var stack []frame
	depth := 0

	top := func() *frame {
		if len(stack) == 0 {
			return nil
		}
		return &stack[len(stack)-1]
	}

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
		tf := top() // capture frame before any pushes this iteration

		// ── Process content at CURRENT depth (before updating) ─────────────

		switch {
		// ── Top level (depth == 0) ─────────────────────────────────────────
		case depth == 0:
			if m := rePackage.FindStringSubmatch(line); m != nil {
				pf.Package = m[1]
			} else if m := reService.FindStringSubmatch(line); m != nil {
				pf.Services = append(pf.Services, ServiceDef{Name: m[1]})
				stack = append(stack, frame{kind: frameKindService, svcIdx: len(pf.Services) - 1, pushDepth: depth})
			} else if m := reMessage.FindStringSubmatch(line); m != nil {
				pf.Messages = append(pf.Messages, MessageDef{Name: m[1]})
				stack = append(stack, frame{kind: frameKindMessage, msgIdx: len(pf.Messages) - 1, pushDepth: depth})
			}

		// ── Inside a service (depth == 1) ──────────────────────────────────
		case depth == 1 && tf != nil && tf.kind == frameKindService:
			if m := reRPC.FindStringSubmatch(line); m != nil {
				suffix := m[4]
				// strip trailing { and ; from suffix
				suffix = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimRight(suffix, "{")), ";"))

				rpc := RPCDef{
					Name:         m[1],
					RequestType:  m[2],
					ResponseType: normalizeType(m[3]),
					Internal:     strings.Contains(suffix, "Internal"),
				}
				// Inline route annotation (our custom syntax: METHOD /path)
				if rm := reRoute.FindStringSubmatch(suffix); rm != nil {
					rpc.HTTPMethod = HTTPMethod(rm[1])
					rpc.HTTPPath = rm[2]
				}

				svc := &pf.Services[tf.svcIdx]
				svc.RPCs = append(svc.RPCs, rpc)
				rpcIdx := len(svc.RPCs) - 1

				// If this line opens a block (options follow), push rpc_options frame.
				if opens > closes {
					stack = append(stack, frame{
						kind: frameKindRPCOptions, svcIdx: tf.svcIdx, rpcIdx: rpcIdx, pushDepth: depth,
					})
				}
			}

		// ── Inside RPC options block (depth == 2) ──────────────────────────
		case depth == 2 && tf != nil && tf.kind == frameKindRPCOptions:
			if reHTTPOption.MatchString(line) {
				stack = append(stack, frame{
					kind: frameKindHTTPOption, svcIdx: tf.svcIdx, rpcIdx: tf.rpcIdx, pushDepth: depth,
				})
				// Also handle single-line option blocks: option (...) = { delete: "..." };
				rpc := &pf.Services[tf.svcIdx].RPCs[tf.rpcIdx]
				if m := reHTTPMethodOpt.FindStringSubmatch(line); m != nil {
					rpc.HTTPMethod = HTTPMethod(strings.ToUpper(m[1]))
					rpc.HTTPPath = m[2]
				}
				if m := reBodyOpt.FindStringSubmatch(line); m != nil {
					rpc.HTTPBody = m[1]
				}
			}

		// ── Inside google.api.http option block (depth == 3) ───────────────
		case depth == 3 && tf != nil && tf.kind == frameKindHTTPOption:
			rpc := &pf.Services[tf.svcIdx].RPCs[tf.rpcIdx]
			if m := reHTTPMethodOpt.FindStringSubmatch(line); m != nil {
				rpc.HTTPMethod = HTTPMethod(strings.ToUpper(m[1]))
				rpc.HTTPPath = m[2]
			}
			if m := reBodyOpt.FindStringSubmatch(line); m != nil {
				rpc.HTTPBody = m[1]
			}

		// ── Inside a message (depth == 1) ──────────────────────────────────
		case depth == 1 && tf != nil && tf.kind == frameKindMessage:
			if m := reField.FindStringSubmatch(line); m != nil {
				modifier := protoModifier(strings.TrimSpace(m[1]))
				fieldType := m[2]
				fieldName := m[3]
				fieldNum, _ := strconv.Atoi(m[4])

				// skip proto keywords mistaken as type
				if protoModifier(fieldType) == modifierOptional || protoModifier(fieldType) == modifierRepeated {
					continue
				}

				msg := &pf.Messages[tf.msgIdx]
				msg.Fields = append(msg.Fields, FieldDef{
					Name:     fieldName,
					Type:     fieldType,
					Number:   fieldNum,
					Repeated: modifier == modifierRepeated,
				})
			}
		}

		// ── Update depth ────────────────────────────────────────────────────
		depth += opens
		depth -= closes
		if depth < 0 {
			depth = 0
		}

		// ── Pop frames that are now closed ──────────────────────────────────
		for len(stack) > 0 && depth <= stack[len(stack)-1].pushDepth {
			stack = stack[:len(stack)-1]
		}
	}

	return pf, nil
}

// normalizeType converts well-known Google proto types to generator-friendly names.
func normalizeType(t string) string {
	switch t {
	case protoTypeEmpty:
		return "Empty"
	default:
		return t
	}
}
