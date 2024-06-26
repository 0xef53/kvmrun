// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: services/hardware/v1/hardware.proto

package hardware

import (
	context "context"
	fmt "fmt"
	types "github.com/0xef53/kvmrun/api/types"
	proto "github.com/gogo/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type ListPCIRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ListPCIRequest) Reset()         { *m = ListPCIRequest{} }
func (m *ListPCIRequest) String() string { return proto.CompactTextString(m) }
func (*ListPCIRequest) ProtoMessage()    {}
func (*ListPCIRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_9416b3c76f5346c5, []int{0}
}
func (m *ListPCIRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ListPCIRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ListPCIRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ListPCIRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListPCIRequest.Merge(m, src)
}
func (m *ListPCIRequest) XXX_Size() int {
	return m.Size()
}
func (m *ListPCIRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ListPCIRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ListPCIRequest proto.InternalMessageInfo

type ListPCIResponse struct {
	Devices              []*types.PCIDevice `protobuf:"bytes,1,rep,name=devices,proto3" json:"devices,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *ListPCIResponse) Reset()         { *m = ListPCIResponse{} }
func (m *ListPCIResponse) String() string { return proto.CompactTextString(m) }
func (*ListPCIResponse) ProtoMessage()    {}
func (*ListPCIResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_9416b3c76f5346c5, []int{1}
}
func (m *ListPCIResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ListPCIResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ListPCIResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ListPCIResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListPCIResponse.Merge(m, src)
}
func (m *ListPCIResponse) XXX_Size() int {
	return m.Size()
}
func (m *ListPCIResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ListPCIResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ListPCIResponse proto.InternalMessageInfo

func (m *ListPCIResponse) GetDevices() []*types.PCIDevice {
	if m != nil {
		return m.Devices
	}
	return nil
}

func init() {
	proto.RegisterType((*ListPCIRequest)(nil), "kvmrun.api.services.hardware.v1.ListPCIRequest")
	proto.RegisterType((*ListPCIResponse)(nil), "kvmrun.api.services.hardware.v1.ListPCIResponse")
}

func init() {
	proto.RegisterFile("services/hardware/v1/hardware.proto", fileDescriptor_9416b3c76f5346c5)
}

var fileDescriptor_9416b3c76f5346c5 = []byte{
	// 230 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x52, 0x2e, 0x4e, 0x2d, 0x2a,
	0xcb, 0x4c, 0x4e, 0x2d, 0xd6, 0xcf, 0x48, 0x2c, 0x4a, 0x29, 0x4f, 0x2c, 0x4a, 0xd5, 0x2f, 0x33,
	0x84, 0xb3, 0xf5, 0x0a, 0x8a, 0xf2, 0x4b, 0xf2, 0x85, 0xe4, 0xb3, 0xcb, 0x72, 0x8b, 0x4a, 0xf3,
	0xf4, 0x12, 0x0b, 0x32, 0xf5, 0x60, 0xea, 0xf5, 0xe0, 0x6a, 0xca, 0x0c, 0xa5, 0x04, 0x4b, 0x2a,
	0x0b, 0x52, 0x8b, 0xf5, 0xc1, 0x24, 0x44, 0x8f, 0x92, 0x00, 0x17, 0x9f, 0x4f, 0x66, 0x71, 0x49,
	0x80, 0xb3, 0x67, 0x50, 0x6a, 0x61, 0x69, 0x6a, 0x71, 0x89, 0x92, 0x07, 0x17, 0x3f, 0x5c, 0xa4,
	0xb8, 0x20, 0x3f, 0xaf, 0x38, 0x55, 0xc8, 0x94, 0x8b, 0x3d, 0x25, 0x15, 0x6c, 0x9c, 0x04, 0xa3,
	0x02, 0xb3, 0x06, 0xb7, 0x91, 0xb4, 0x1e, 0x92, 0x55, 0x10, 0xe3, 0x02, 0x9c, 0x3d, 0x5d, 0xc0,
	0x6a, 0x82, 0x60, 0x6a, 0x8d, 0xea, 0xb9, 0xf8, 0x3d, 0xa0, 0xb6, 0x07, 0x43, 0x9c, 0x23, 0x94,
	0xc3, 0xc5, 0x0e, 0x35, 0x5c, 0x48, 0x5f, 0x8f, 0x80, 0x73, 0xf5, 0x50, 0x1d, 0x26, 0x65, 0x40,
	0xbc, 0x06, 0x88, 0xbb, 0x9d, 0x3c, 0x4e, 0x3c, 0x92, 0x63, 0xbc, 0xf0, 0x48, 0x8e, 0xf1, 0xc1,
	0x23, 0x39, 0xc6, 0x28, 0xab, 0xf4, 0xcc, 0x92, 0x8c, 0xd2, 0x24, 0xbd, 0xe4, 0xfc, 0x5c, 0x7d,
	0x83, 0x8a, 0xd4, 0x34, 0x53, 0x63, 0x7d, 0x88, 0x81, 0xfa, 0x89, 0x05, 0x99, 0xfa, 0xd8, 0x02,
	0xd8, 0x1a, 0xc6, 0x4e, 0x62, 0x03, 0x87, 0x96, 0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0x21, 0x42,
	0x5b, 0x04, 0x88, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// HardwareServiceClient is the client API for HardwareService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type HardwareServiceClient interface {
	ListPCI(ctx context.Context, in *ListPCIRequest, opts ...grpc.CallOption) (*ListPCIResponse, error)
}

type hardwareServiceClient struct {
	cc *grpc.ClientConn
}

func NewHardwareServiceClient(cc *grpc.ClientConn) HardwareServiceClient {
	return &hardwareServiceClient{cc}
}

func (c *hardwareServiceClient) ListPCI(ctx context.Context, in *ListPCIRequest, opts ...grpc.CallOption) (*ListPCIResponse, error) {
	out := new(ListPCIResponse)
	err := c.cc.Invoke(ctx, "/kvmrun.api.services.hardware.v1.HardwareService/ListPCI", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// HardwareServiceServer is the server API for HardwareService service.
type HardwareServiceServer interface {
	ListPCI(context.Context, *ListPCIRequest) (*ListPCIResponse, error)
}

// UnimplementedHardwareServiceServer can be embedded to have forward compatible implementations.
type UnimplementedHardwareServiceServer struct {
}

func (*UnimplementedHardwareServiceServer) ListPCI(ctx context.Context, req *ListPCIRequest) (*ListPCIResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListPCI not implemented")
}

func RegisterHardwareServiceServer(s *grpc.Server, srv HardwareServiceServer) {
	s.RegisterService(&_HardwareService_serviceDesc, srv)
}

func _HardwareService_ListPCI_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListPCIRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HardwareServiceServer).ListPCI(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/kvmrun.api.services.hardware.v1.HardwareService/ListPCI",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HardwareServiceServer).ListPCI(ctx, req.(*ListPCIRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _HardwareService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "kvmrun.api.services.hardware.v1.HardwareService",
	HandlerType: (*HardwareServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListPCI",
			Handler:    _HardwareService_ListPCI_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "services/hardware/v1/hardware.proto",
}

func (m *ListPCIRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ListPCIRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ListPCIRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	return len(dAtA) - i, nil
}

func (m *ListPCIResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ListPCIResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ListPCIResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Devices) > 0 {
		for iNdEx := len(m.Devices) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Devices[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintHardware(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintHardware(dAtA []byte, offset int, v uint64) int {
	offset -= sovHardware(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *ListPCIRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *ListPCIResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Devices) > 0 {
		for _, e := range m.Devices {
			l = e.Size()
			n += 1 + l + sovHardware(uint64(l))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovHardware(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozHardware(x uint64) (n int) {
	return sovHardware(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *ListPCIRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowHardware
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ListPCIRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ListPCIRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipHardware(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthHardware
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ListPCIResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowHardware
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ListPCIResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ListPCIResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Devices", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowHardware
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthHardware
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthHardware
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Devices = append(m.Devices, &types.PCIDevice{})
			if err := m.Devices[len(m.Devices)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipHardware(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthHardware
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipHardware(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowHardware
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowHardware
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowHardware
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthHardware
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupHardware
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthHardware
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthHardware        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowHardware          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupHardware = fmt.Errorf("proto: unexpected end of group")
)
