package chaintools

import (
	"context"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/hdkeychain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/runtime/protoimpl"
)

type GetBip44HdAddressRequestPrompt struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Xpub         string `protobuf:"bytes,1,opt,name=xpub,proto3" json:"xpub,omitempty"`
	Change       bool   `protobuf:"varint,2,opt,name=change,proto3" json:"change,omitempty"`
	AddressIndex uint32 `protobuf:"varint,3,opt,name=address_index,json=addressIndex,proto3" json:"address_index,omitempty"`
}

type GetBip44HdAddressResponsePrompt struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PubKey   []byte `protobuf:"bytes,1,opt,name=pub_key,json=pubKey,proto3" json:"pub_key,omitempty"`
	CashAddr string `protobuf:"bytes,2,opt,name=cash_addr,json=cashAddr,proto3" json:"cash_addr,omitempty"`
	SlpAddr  string `protobuf:"bytes,3,opt,name=slp_addr,json=slpAddr,proto3" json:"slp_addr,omitempty"`
}

type ChainTools struct {
	chainParams *chaincfg.Params
	//slpIndex  *indexers.SlpIndex
	//enableSlpIndex bool
}

func NewChainTools() (tools *ChainTools, err error) {
	tools = &ChainTools{
		//App: app,
		chainParams: &chaincfg.MainNetParams,
		//enableSlpIndex: viper.GetBool("BitcoinCash.EnableSlpIndex"),
	}

	return tools, nil
}

// GetBip44HdAddressPrompt this method will return an address based on the requested HD path
func (s *ChainTools) GetBip44HdAddressPrompt(ctx context.Context, req *GetBip44HdAddressRequestPrompt) (*GetBip44HdAddressResponsePrompt, error) {
	xpub := req.Xpub
	if len(xpub) < 4 {
		return nil, status.Error(codes.InvalidArgument, "xpub is missing in request")
	}

	if xpub[:4] != "xpub" {
		return nil, status.Error(codes.InvalidArgument, "xpub provided does not start with 'xpub'")
	}

	masterKey, err := hdkeychain.NewKeyFromString(xpub)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid xpub: %v", err)
	}

	var change uint32 = 0
	if req.Change {
		change = 1
	}

	ext, err := masterKey.Child(change)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid xpub: %v", err)
	}

	extK, err := ext.Child(req.AddressIndex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid xpub: %v", err)
	}

	pubKey, err := extK.ECPubKey()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	addr, err := extK.Address(s.chainParams)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	slpAddrStr := ""
	//if s.slpIndex != nil {
	//if s.enableSlpIndex == true {
	//	slpAddr, err := bchutil.NewSlpAddressPubKeyHash(addr.Hash160()[:], s.chainParams)
	//	if err != nil {
	//		return nil, status.Errorf(codes.Internal, "failed to create slp pubkeyhash address from hash160: %v", err)
	//	}
	//	slpAddrStr = slpAddr.EncodeAddress()
	//}

	res := &GetBip44HdAddressResponsePrompt{
		PubKey:   pubKey.SerializeCompressed(),
		CashAddr: addr.EncodeAddress(),
		SlpAddr:  slpAddrStr,
	}

	return res, nil
}

func (s *ChainTools) NewToOldAddress(newAddress string) (string, error) {
	addr1, err := bchutil.DecodeAddress(newAddress, s.chainParams)
	if err != nil {
		return "", err
	}
	addr2, err := bchutil.NewLegacyAddressPubKeyHash(addr1.ScriptAddress(), s.chainParams)
	if err != nil {
		return "", err
	}
	return addr2.EncodeAddress(), nil
}
