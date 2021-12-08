// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2015 The Go Authors.  All rights reserved.
// https://github.com/golang/protobuf
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Package grpc outputs gRPC service descriptions in Go code.
// It runs as a plugin for the Go protocol buffer compiler plugin.
// It is linked in to protoc-gen-go.
package grpc

import (
	"fmt"
	"strconv"
	"strings"

	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// generatedCodeVersion indicates a version of the generated code.
// It is incremented whenever an incompatibility between the generated code and
// the grpc package is introduced; the generated code references
// a constant, grpc.SupportPackageIsVersionN (where N is generatedCodeVersion).
const generatedCodeVersion = 4

// Paths for packages used by code generated in this file,
// relative to the import_prefix of the generator.Generator.
const (
	contextPkgPath = "context"
	grpcPkgPath    = "google.golang.org/grpc"
	codePkgPath    = "google.golang.org/grpc/codes"
	statusPkgPath  = "google.golang.org/grpc/status"
)

func init() {
	generator.RegisterPlugin(new(grpc))
}

// grpc is an implementation of the Go protocol buffer compiler's
// plugin architecture.  It generates bindings for gRPC support.
type grpc struct {
	gen *generator.Generator
}

// Name returns the name of this plugin, "grpc".
func (g *grpc) Name() string {
	return "grpc"
}

// The names for packages imported in the generated code.
// They may vary from the final path component of the import path
// if the name is used by other packages.
var (
	contextPkg string
	grpcPkg    string
)

// Init initializes the plugin.
func (g *grpc) Init(gen *generator.Generator) {
	g.gen = gen
}

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (g *grpc) objectNamed(name string) generator.Object {
	g.gen.RecordTypeUse(name)
	return g.gen.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (g *grpc) typeName(str string) string {
	return g.gen.TypeName(g.objectNamed(str))
}

// P forwards to g.gen.P.
func (g *grpc) P(args ...interface{}) { g.gen.P(args...) }

// Generate generates code for the services in the given file.
func (g *grpc) Generate(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	contextPkg = string(g.gen.AddImport(contextPkgPath))
	grpcPkg = string(g.gen.AddImport(grpcPkgPath))

	g.P("// Reference imports to suppress errors if they are not otherwise used.")
	g.P("var _ ", contextPkg, ".Context")
	g.P("var _ ", grpcPkg, ".ClientConn")
	g.P()

	// Assert version compatibility.
	g.P("// This is a compile-time assertion to ensure that this generated file")
	g.P("// is compatible with the grpc package it is being compiled against.")
	g.P("const _ = ", grpcPkg, ".SupportPackageIsVersion", generatedCodeVersion)
	g.P()

	for i, service := range file.FileDescriptorProto.Service {
		g.generateService(file, service, i)
	}
}

// GenerateImports generates the import declaration for this file.
func (g *grpc) GenerateImports(file *generator.FileDescriptor) {
}

// reservedClientName records whether a client name is reserved on the client side.
var reservedClientName = map[string]bool{
	// TODO: do we need any in gRPC?
}

func unexport(s string) string { return strings.ToLower(s[:1]) + s[1:] }

// deprecationComment is the standard comment added to deprecated
// messages, fields, enums, and enum values.
var deprecationComment = "// Deprecated: Do not use."

func (g *grpc) generateSwitch(service *pb.ServiceDescriptorProto) {
	templateBegin := `
	/* //for pb_gen_switch.go begin
	package pb_gen_switch
	
	import (
		"github.com/golang/protobuf/proto"
		"my_logic/internal/logic_interface"
		"my_logic/pb_gen"
	)
	
	func GameMessage(l logic_interface.LogicBase, in *pb_gen.GameRequest) (*pb_gen.GameResponse, error) {
		response := &pb_gen.GameResponse{}
	
		switch in.MsgID {

			`
	templateEnd := `
		default:
			l.Errorf("un supported pt: %v ", in.MsgID)
	
		}
		return response, nil
	}
	
	//for pb_gen_switch.go end */`

	templateBody := `
case __PTID__:
	nowIn := __INPUT_TYPE__{}
	err := nowIn.XXX_Unmarshal(in.Data)
	if err != nil {
		l.Errorf(".__INPUT_TYPE__ Unmarshal failed: %v roleid: %s accid: %s", err, sp.roleid, sp.accid)
		return nil, err
	}
	var resp *__OUTPUT_TYPE__
	var opLog *pb_gen.OpLog
	//func (l *GameMessageLogic) __METHOD_NAME__(in *__INPUT_TYPE__) (*__OUTPUT_TYPE__, *pb_gen.OpLog, error)
	resp, opLog, err = l.__METHOD_NAME__(&nowIn)
	if err != nil {
		l.Errorf("__METHOD_NAME__ raw failed: %v roleid: %s accid: %s", err, sp.roleid, sp.accid)
		return nil, err
	}
	response.MsgId = __PTID__
	response.Data, err = proto.Marshal(resp)
	if err != nil {
		l.Errorf("__METHOD_NAME__ marshal Response failed: %v roleid: %s accid: %s", err, sp.roleid, sp.accid)
		return nil, err
	}

	if opLog != nil{
		response.OpLog, err = proto.Marshal(opLog)
		if err != nil {
			l.Errorf("__METHOD_NAME__ marshal OpLog failed: %v roleid: %s accid: %s", err, sp.roleid, sp.accid)
			return nil, err
		}
	}

	return response, nil	
	`
	//fmt.Println(templateBody)
	g.P(templateBegin)
	for _, method := range service.Method {
		it := strings.Replace(*method.InputType, ".", "", 1)
		ot := strings.Replace(*method.OutputType, ".", "", 1)
		line := strings.ReplaceAll(templateBody, "__INPUT_TYPE__", it)
		line = strings.ReplaceAll(line, "__OUTPUT_TYPE__", ot)
		line = strings.ReplaceAll(line, "__METHOD_NAME__", *method.Name)
		ary := strings.Split(method.Options.String(), ":")
		//optstr := fmt.Sprintf("%s_%d", method.Options.String(), len(ary))
		if len(ary) >= 2 {
			ptid := strings.ReplaceAll(ary[1], " ", "")
			//ptid = fmt.Sprintf("%s_%s_%s", *method.InputType, *method.OutputType, *method.Name)
			line = strings.ReplaceAll(line, "__PTID__", ptid)
		}
		g.P(line)
		//fmt.Println(line)
	}
	g.P(templateEnd)

	//generate interface api
	templateInterfaceBegin := `
	/* //for logic_interface begin
	package logic_interface

	import (
		"github.com/tal-tech/go-zero/core/logx"
		"my_logic/pb_gen"
	)
	
	type LogicBase interface {`
	templateInterfaceEnd := `logx.Logger
}
//for logic_interface end */`
	templateInterfaceBody := `	__METHOD_NAME__(in *__INPUT_TYPE__) (*__OUTPUT_TYPE__, *pb_gen.OpLog, error)`
	g.P(templateInterfaceBegin)
	for _, method := range service.Method {
		it := strings.Replace(*method.InputType, ".", "", 1)
		ot := strings.Replace(*method.OutputType, ".", "", 1)
		line := strings.ReplaceAll(templateInterfaceBody, "__INPUT_TYPE__", it)
		line = strings.ReplaceAll(line, "__OUTPUT_TYPE__", ot)
		line = strings.ReplaceAll(line, "__METHOD_NAME__", *method.Name)
		g.P(line)
	}
	g.P(templateInterfaceEnd)

}

func (g *grpc) generatePushFuncs(service *pb.ServiceDescriptorProto) {

	templateBegin := `/* //for logic_push begin
	package svc

	import (
			"fmt"
			"github.com/golang/protobuf/proto"
			"my_logic/internal/utils"
			"my_logic/pb_gen"
	)

	`

	templateBody := `
	func (l *ServiceContext) __METHOD_NAME__(roleUID string, msg *__PB_TYPE__) {
	if msg != nil {
			bin, err := proto.Marshal(msg)
			if err != nil {
					utils.DoAlarm(fmt.Sprintf("ServiceContext __METHOD_NAME__ failed, err: %s roleUID: %s", err.Error(), roleUID) )
					return
			}
			l.SendPlayer(roleUID, uint32(__PTID__), bin)
			}else{
			l.SendPlayer(roleUID, uint32(__PTID__), nil)
			}
	}       
	`
	templateEnd := `
//for logic_push end */`
	g.P(templateBegin)
	for _, method := range service.Method {
		it := strings.Replace(*method.InputType, ".", "", 1)
		//ot := strings.Replace(*method.OutputType, ".", "", 1)
		line := strings.ReplaceAll(templateBody, "__METHOD_NAME__", *method.Name)
		line = strings.ReplaceAll(line, "__PB_TYPE__", it)

		ary := strings.Split(method.Options.String(), ":")
		if len(ary) >= 2 {
			ptid := strings.ReplaceAll(ary[1], " ", "")
			line = strings.ReplaceAll(line, "__PTID__", ptid)
		}
		g.P(line)
	}
	g.P(templateEnd)
}

func (g *grpc) generateSwitchV1(service *pb.ServiceDescriptorProto) {
	templateBegin := `/* // for switch_process begin
	package pb_gen_switch

	import (
		"github.com/golang/protobuf/proto"
		"my_logic/internal/logic_interface"
		"my_logic/internal/utils"
		"my_logic/pb_gen"
		"fmt"
		"runtime"
	)
	
	func (s *SwitchMgr) StartProceeByIdx(idx int) {
		var ll logic_interface.LogicBase
		defer func() {
			if e := recover(); e != nil {
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				ll.Errorf("RECOVER:", e)
				ll.Errorf("STACK:%d", idx)
				ll.Errorf(string(buf[:n]))
				go s.StartProceeByIdx(idx)
			}
		}()
		for {
			ch := s.chary[idx]
			select {
			case sp := <-ch:
				l := sp.lb
				ll = l
				switch sp.in.MsgID {`
	templateEnd := ` 
default:
	// check role token
	l.Errorf("un supported pt:", sp.in.MsgID)
	utils.DoAlarm(fmt.Sprintf("un supported pt:", sp.in.MsgID))
	sp.Done(fmt.Errorf("un supported pt:", sp.in.MsgID))
}
}
}
}


func (s *SwitchMgr) Init() {
s.chary = make([]chan switchProcess, s.threadNum)
for i := 0; i < s.threadNum; i++ {
s.chary[i] = make(chan switchProcess,1024)
}

for i := 0; i < s.threadNum; i++ {
go s.StartProceeByIdx(i)
}
}
// for switch_process end */`

	templateBody := `case __PTID__:
	nowIn := __INPUT_TYPE__{}
	err := nowIn.XXX_Unmarshal(sp.in.Data)
	if err != nil {
		l.Errorf("__INPUT_TYPE__ Unmarshal failed: %v roleid: %s accid: %s", err, sp.roleid, sp.accid)
		sp.Done(err)
		continue
	}
	var resp *__OUTPUT_TYPE__
	var opLog *pb_gen.OpLog
	resp, opLog, err = l.__METHOD_NAME__(&nowIn)
	if err != nil {
		l.Errorf("__METHOD_NAME__ raw failed: %v roleid: %s accid: %s", err, sp.roleid, sp.accid)
		sp.Done(err)
		continue
	}
	sp.out.MsgId = __PTID__
	sp.out.Data, err = proto.Marshal(resp)
	if err != nil {
		l.Errorf("__METHOD_NAME__ marshal Response failed: %v roleid: %s accid: %s", err, sp.roleid, sp.accid)
		sp.Done(err)
		continue
	}

	if opLog != nil {
		sp.out.OpLog, err = proto.Marshal(opLog)
		if err != nil {
			l.Errorf("__METHOD_NAME__ marshal OpLog failed: %v roleid: %s accid: %s", err, sp.roleid, sp.accid)
			sp.Done(err)
			continue
		}
	}
	sp.Done(nil)`
	g.P(templateBegin)
	for _, method := range service.Method {
		it := strings.Replace(*method.InputType, ".", "", 1)
		ot := strings.Replace(*method.OutputType, ".", "", 1)
		line := strings.ReplaceAll(templateBody, "__INPUT_TYPE__", it)
		line = strings.ReplaceAll(line, "__OUTPUT_TYPE__", ot)
		line = strings.ReplaceAll(line, "__METHOD_NAME__", *method.Name)
		ary := strings.Split(method.Options.String(), ":")
		//optstr := fmt.Sprintf("%s_%d", method.Options.String(), len(ary))
		if len(ary) >= 2 {
			ptid := strings.ReplaceAll(ary[1], " ", "")
			//ptid = fmt.Sprintf("%s_%s_%s", *method.InputType, *method.OutputType, *method.Name)
			line = strings.ReplaceAll(line, "__PTID__", ptid)
		}
		g.P(line)
		//fmt.Println(line)
	}
	g.P(templateEnd)

	//for interface
	//generate interface api
	templateInterfaceBegin := `
/* //for logic_interface begin
package logic_interface

import (
	"context"
	"github.com/tal-tech/go-zero/core/logx"
	"my_logic/pb_gen"
)

type LogicBase interface {`
	templateInterfaceEnd := `logx.Logger
}
//for logic_interface end */`
	templateInterfaceBody := `	__METHOD_NAME__(in *__INPUT_TYPE__) (*__OUTPUT_TYPE__, *pb_gen.OpLog, error)`
	g.P(templateInterfaceBegin)
	for _, method := range service.Method {
		it := strings.Replace(*method.InputType, ".", "", 1)
		ot := strings.Replace(*method.OutputType, ".", "", 1)
		line := strings.ReplaceAll(templateInterfaceBody, "__INPUT_TYPE__", it)
		line = strings.ReplaceAll(line, "__OUTPUT_TYPE__", ot)
		line = strings.ReplaceAll(line, "__METHOD_NAME__", *method.Name)
		g.P(line)
	}
	g.P("	GetCtx() context.Context")
	g.P(templateInterfaceEnd)
}

// generateService generates all the code for the named service.
func (g *grpc) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	path := fmt.Sprintf("6,%d", index) // 6 means service.
	if service.GetName() == "PushPbService" {
		g.generatePushFuncs(service)
		return
	}
	if service.GetName() == "LoginPbService" {
		//g.generateSwitch(service)
		g.generateSwitchV1(service)
		return
	}
	origServName := service.GetName()
	fullServName := origServName
	if pkg := file.GetPackage(); pkg != "" {
		fullServName = pkg + "." + fullServName
	}
	servName := generator.CamelCase(origServName)
	deprecated := service.GetOptions().GetDeprecated()

	g.P()
	g.P(fmt.Sprintf(`// %sClient is the client API for %s service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.`, servName, servName))

	// Client interface.
	if deprecated {
		g.P("//")
		g.P(deprecationComment)
	}
	g.P("type ", servName, "Client interface {")
	for i, method := range service.Method {
		g.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		g.P(g.generateClientSignature(servName, method))
	}
	g.P("}")
	g.P()

	// Client structure.
	g.P("type ", unexport(servName), "Client struct {")
	g.P("cc *", grpcPkg, ".ClientConn")
	g.P("}")
	g.P()

	// NewClient factory.
	if deprecated {
		g.P(deprecationComment)
	}
	g.P("func New", servName, "Client (cc *", grpcPkg, ".ClientConn) ", servName, "Client {")
	g.P("return &", unexport(servName), "Client{cc}")
	g.P("}")
	g.P()

	var methodIndex, streamIndex int
	serviceDescVar := "_" + servName + "_serviceDesc"
	// Client method implementations.
	for _, method := range service.Method {
		var descExpr string
		if !method.GetServerStreaming() && !method.GetClientStreaming() {
			// Unary RPC method
			descExpr = fmt.Sprintf("&%s.Methods[%d]", serviceDescVar, methodIndex)
			methodIndex++
		} else {
			// Streaming RPC method
			descExpr = fmt.Sprintf("&%s.Streams[%d]", serviceDescVar, streamIndex)
			streamIndex++
		}
		g.generateClientMethod(servName, fullServName, serviceDescVar, method, descExpr)
	}

	// Server interface.
	serverType := servName + "Server"
	g.P("// ", serverType, " is the server API for ", servName, " service.")
	if deprecated {
		g.P("//")
		g.P(deprecationComment)
	}
	g.P("type ", serverType, " interface {")
	for i, method := range service.Method {
		g.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		g.P(g.generateServerSignature(servName, method))
	}
	g.P("}")
	g.P()

	// Server Unimplemented struct for forward compatability.
	if deprecated {
		g.P(deprecationComment)
	}
	g.generateUnimplementedServer(servName, service)

	// Server registration.
	if deprecated {
		g.P(deprecationComment)
	}
	g.P("func Register", servName, "Server(s *", grpcPkg, ".Server, srv ", serverType, ") {")
	g.P("s.RegisterService(&", serviceDescVar, `, srv)`)
	g.P("}")
	g.P()

	// Server handler implementations.
	var handlerNames []string
	for _, method := range service.Method {
		hname := g.generateServerMethod(servName, fullServName, method)
		handlerNames = append(handlerNames, hname)
	}

	// Service descriptor.
	g.P("var ", serviceDescVar, " = ", grpcPkg, ".ServiceDesc {")
	g.P("ServiceName: ", strconv.Quote(fullServName), ",")
	g.P("HandlerType: (*", serverType, ")(nil),")
	g.P("Methods: []", grpcPkg, ".MethodDesc{")
	for i, method := range service.Method {
		if method.GetServerStreaming() || method.GetClientStreaming() {
			continue
		}
		g.P("{")
		g.P("MethodName: ", strconv.Quote(method.GetName()), ",")
		g.P("Handler: ", handlerNames[i], ",")
		g.P("},")
	}
	g.P("},")
	g.P("Streams: []", grpcPkg, ".StreamDesc{")
	for i, method := range service.Method {
		if !method.GetServerStreaming() && !method.GetClientStreaming() {
			continue
		}
		g.P("{")
		g.P("StreamName: ", strconv.Quote(method.GetName()), ",")
		g.P("Handler: ", handlerNames[i], ",")
		if method.GetServerStreaming() {
			g.P("ServerStreams: true,")
		}
		if method.GetClientStreaming() {
			g.P("ClientStreams: true,")
		}
		g.P("},")
	}
	g.P("},")
	g.P("Metadata: \"", file.GetName(), "\",")
	g.P("}")
	g.P()
}

// generateUnimplementedServer creates the unimplemented server struct
func (g *grpc) generateUnimplementedServer(servName string, service *pb.ServiceDescriptorProto) {
	serverType := servName + "Server"
	g.P("// Unimplemented", serverType, " can be embedded to have forward compatible implementations.")
	g.P("type Unimplemented", serverType, " struct {")
	g.P("}")
	g.P()
	// Unimplemented<service_name>Server's concrete methods
	for _, method := range service.Method {
		g.generateServerMethodConcrete(servName, method)
	}
	g.P()
}

// generateServerMethodConcrete returns unimplemented methods which ensure forward compatibility
func (g *grpc) generateServerMethodConcrete(servName string, method *pb.MethodDescriptorProto) {
	header := g.generateServerSignatureWithParamNames(servName, method)
	g.P("func (*Unimplemented", servName, "Server) ", header, " {")
	var nilArg string
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		nilArg = "nil, "
	}
	methName := generator.CamelCase(method.GetName())
	statusPkg := string(g.gen.AddImport(statusPkgPath))
	codePkg := string(g.gen.AddImport(codePkgPath))
	g.P("return ", nilArg, statusPkg, `.Errorf(`, codePkg, `.Unimplemented, "method `, methName, ` not implemented")`)
	g.P("}")
}

// generateClientSignature returns the client-side signature for a method.
func (g *grpc) generateClientSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}
	reqArg := ", in *" + g.typeName(method.GetInputType())
	if method.GetClientStreaming() {
		reqArg = ""
	}
	respName := "*" + g.typeName(method.GetOutputType())
	if method.GetServerStreaming() || method.GetClientStreaming() {
		respName = servName + "_" + generator.CamelCase(origMethName) + "Client"
	}
	return fmt.Sprintf("%s(ctx %s.Context%s, opts ...%s.CallOption) (%s, error)", methName, contextPkg, reqArg, grpcPkg, respName)
}

func (g *grpc) generateClientMethod(servName, fullServName, serviceDescVar string, method *pb.MethodDescriptorProto, descExpr string) {
	sname := fmt.Sprintf("/%s/%s", fullServName, method.GetName())
	methName := generator.CamelCase(method.GetName())
	inType := g.typeName(method.GetInputType())
	outType := g.typeName(method.GetOutputType())

	if method.GetOptions().GetDeprecated() {
		g.P(deprecationComment)
	}
	g.P("func (c *", unexport(servName), "Client) ", g.generateClientSignature(servName, method), "{")
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		g.P("out := new(", outType, ")")
		// TODO: Pass descExpr to Invoke.
		g.P(`err := c.cc.Invoke(ctx, "`, sname, `", in, out, opts...)`)
		g.P("if err != nil { return nil, err }")
		g.P("return out, nil")
		g.P("}")
		g.P()
		return
	}
	streamType := unexport(servName) + methName + "Client"
	g.P("stream, err := c.cc.NewStream(ctx, ", descExpr, `, "`, sname, `", opts...)`)
	g.P("if err != nil { return nil, err }")
	g.P("x := &", streamType, "{stream}")
	if !method.GetClientStreaming() {
		g.P("if err := x.ClientStream.SendMsg(in); err != nil { return nil, err }")
		g.P("if err := x.ClientStream.CloseSend(); err != nil { return nil, err }")
	}
	g.P("return x, nil")
	g.P("}")
	g.P()

	genSend := method.GetClientStreaming()
	genRecv := method.GetServerStreaming()
	genCloseAndRecv := !method.GetServerStreaming()

	// Stream auxiliary types and methods.
	g.P("type ", servName, "_", methName, "Client interface {")
	if genSend {
		g.P("Send(*", inType, ") error")
	}
	if genRecv {
		g.P("Recv() (*", outType, ", error)")
	}
	if genCloseAndRecv {
		g.P("CloseAndRecv() (*", outType, ", error)")
	}
	g.P(grpcPkg, ".ClientStream")
	g.P("}")
	g.P()

	g.P("type ", streamType, " struct {")
	g.P(grpcPkg, ".ClientStream")
	g.P("}")
	g.P()

	if genSend {
		g.P("func (x *", streamType, ") Send(m *", inType, ") error {")
		g.P("return x.ClientStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genRecv {
		g.P("func (x *", streamType, ") Recv() (*", outType, ", error) {")
		g.P("m := new(", outType, ")")
		g.P("if err := x.ClientStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}
	if genCloseAndRecv {
		g.P("func (x *", streamType, ") CloseAndRecv() (*", outType, ", error) {")
		g.P("if err := x.ClientStream.CloseSend(); err != nil { return nil, err }")
		g.P("m := new(", outType, ")")
		g.P("if err := x.ClientStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}
}

// generateServerSignatureWithParamNames returns the server-side signature for a method with parameter names.
func (g *grpc) generateServerSignatureWithParamNames(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}

	var reqArgs []string
	ret := "error"
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "ctx "+contextPkg+".Context")
		ret = "(*" + g.typeName(method.GetOutputType()) + ", error)"
	}
	if !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "req *"+g.typeName(method.GetInputType()))
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		reqArgs = append(reqArgs, "srv "+servName+"_"+generator.CamelCase(origMethName)+"Server")
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

// generateServerSignature returns the server-side signature for a method.
func (g *grpc) generateServerSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}

	var reqArgs []string
	ret := "error"
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		reqArgs = append(reqArgs, contextPkg+".Context")
		ret = "(*" + g.typeName(method.GetOutputType()) + ", error)"
	}
	if !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "*"+g.typeName(method.GetInputType()))
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		reqArgs = append(reqArgs, servName+"_"+generator.CamelCase(origMethName)+"Server")
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func (g *grpc) generateServerMethod(servName, fullServName string, method *pb.MethodDescriptorProto) string {
	methName := generator.CamelCase(method.GetName())
	hname := fmt.Sprintf("_%s_%s_Handler", servName, methName)
	inType := g.typeName(method.GetInputType())
	outType := g.typeName(method.GetOutputType())

	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		g.P("func ", hname, "(srv interface{}, ctx ", contextPkg, ".Context, dec func(interface{}) error, interceptor ", grpcPkg, ".UnaryServerInterceptor) (interface{}, error) {")
		g.P("in := new(", inType, ")")
		g.P("if err := dec(in); err != nil { return nil, err }")
		g.P("if interceptor == nil { return srv.(", servName, "Server).", methName, "(ctx, in) }")
		g.P("info := &", grpcPkg, ".UnaryServerInfo{")
		g.P("Server: srv,")
		g.P("FullMethod: ", strconv.Quote(fmt.Sprintf("/%s/%s", fullServName, methName)), ",")
		g.P("}")
		g.P("handler := func(ctx ", contextPkg, ".Context, req interface{}) (interface{}, error) {")
		g.P("return srv.(", servName, "Server).", methName, "(ctx, req.(*", inType, "))")
		g.P("}")
		g.P("return interceptor(ctx, in, info, handler)")
		g.P("}")
		g.P()
		return hname
	}
	streamType := unexport(servName) + methName + "Server"
	g.P("func ", hname, "(srv interface{}, stream ", grpcPkg, ".ServerStream) error {")
	if !method.GetClientStreaming() {
		g.P("m := new(", inType, ")")
		g.P("if err := stream.RecvMsg(m); err != nil { return err }")
		g.P("return srv.(", servName, "Server).", methName, "(m, &", streamType, "{stream})")
	} else {
		g.P("return srv.(", servName, "Server).", methName, "(&", streamType, "{stream})")
	}
	g.P("}")
	g.P()

	genSend := method.GetServerStreaming()
	genSendAndClose := !method.GetServerStreaming()
	genRecv := method.GetClientStreaming()

	// Stream auxiliary types and methods.
	g.P("type ", servName, "_", methName, "Server interface {")
	if genSend {
		g.P("Send(*", outType, ") error")
	}
	if genSendAndClose {
		g.P("SendAndClose(*", outType, ") error")
	}
	if genRecv {
		g.P("Recv() (*", inType, ", error)")
	}
	g.P(grpcPkg, ".ServerStream")
	g.P("}")
	g.P()

	g.P("type ", streamType, " struct {")
	g.P(grpcPkg, ".ServerStream")
	g.P("}")
	g.P()

	if genSend {
		g.P("func (x *", streamType, ") Send(m *", outType, ") error {")
		g.P("return x.ServerStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genSendAndClose {
		g.P("func (x *", streamType, ") SendAndClose(m *", outType, ") error {")
		g.P("return x.ServerStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genRecv {
		g.P("func (x *", streamType, ") Recv() (*", inType, ", error) {")
		g.P("m := new(", inType, ")")
		g.P("if err := x.ServerStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}

	return hname
}
