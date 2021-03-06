// Copyright (C) 2014 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package table

import (
	//"fmt"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

func TestDestinationNewIPv4(t *testing.T) {
	peerD := DestCreatePeer()
	pathD := DestCreatePath(peerD)
	ipv4d := NewDestination(pathD[0].GetNlri())
	assert.NotNil(t, ipv4d)
}
func TestDestinationNewIPv6(t *testing.T) {
	peerD := DestCreatePeer()
	pathD := DestCreatePath(peerD)
	ipv6d := NewDestination(pathD[0].GetNlri())
	assert.NotNil(t, ipv6d)
}

func TestDestinationSetRouteFamily(t *testing.T) {
	dd := &Destination{}
	dd.setRouteFamily(bgp.RF_IPv4_UC)
	rf := dd.Family()
	assert.Equal(t, rf, bgp.RF_IPv4_UC)
}
func TestDestinationGetRouteFamily(t *testing.T) {
	dd := &Destination{}
	dd.setRouteFamily(bgp.RF_IPv6_UC)
	rf := dd.Family()
	assert.Equal(t, rf, bgp.RF_IPv6_UC)
}
func TestDestinationSetNlri(t *testing.T) {
	dd := &Destination{}
	nlri := bgp.NewIPAddrPrefix(24, "13.2.3.1")
	dd.setNlri(nlri)
	r_nlri := dd.GetNlri()
	assert.Equal(t, r_nlri, nlri)
}
func TestDestinationGetNlri(t *testing.T) {
	dd := &Destination{}
	nlri := bgp.NewIPAddrPrefix(24, "10.110.123.1")
	dd.setNlri(nlri)
	r_nlri := dd.GetNlri()
	assert.Equal(t, r_nlri, nlri)
}

func TestCalculate(t *testing.T) {
	origin := bgp.NewPathAttributeOrigin(0)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{65001})}
	aspath := bgp.NewPathAttributeAsPath(aspathParam)
	nexthop := bgp.NewPathAttributeNextHop("10.0.0.1")
	med := bgp.NewPathAttributeMultiExitDisc(0)
	pathAttributes := []bgp.PathAttributeInterface{origin, aspath, nexthop, med}
	nlri := bgp.NewIPAddrPrefix(24, "10.10.0.101")
	updateMsg := bgp.NewBGPUpdateMessage(nil, pathAttributes, []*bgp.IPAddrPrefix{nlri})
	peer1 := &PeerInfo{AS: 1, Address: net.IP{1, 1, 1, 1}}
	path1 := ProcessMessage(updateMsg, peer1, time.Now())[0]
	path1.Filter("1", POLICY_DIRECTION_IMPORT)

	action := &AsPathPrependAction{
		asn:    100,
		repeat: 10,
	}

	path2 := action.Apply(path1.Clone(false), nil)
	path1.Filter("2", POLICY_DIRECTION_IMPORT)
	path2.Filter("1", POLICY_DIRECTION_IMPORT)

	d := NewDestination(nlri)
	d.addNewPath(path1)
	d.addNewPath(path2)

	d.Calculate([]string{"1", "2"})

	assert.Equal(t, len(d.GetKnownPathList("1")), 0)
	assert.Equal(t, len(d.GetKnownPathList("2")), 1)
	assert.Equal(t, len(d.knownPathList), 2)

	d.addWithdraw(path1.Clone(true))

	d.Calculate([]string{"1", "2"})

	assert.Equal(t, len(d.GetKnownPathList("1")), 0)
	assert.Equal(t, len(d.GetKnownPathList("2")), 0)
	assert.Equal(t, len(d.knownPathList), 0)
}

func TestCalculate2(t *testing.T) {

	origin := bgp.NewPathAttributeOrigin(0)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{65001})}
	aspath := bgp.NewPathAttributeAsPath(aspathParam)
	nexthop := bgp.NewPathAttributeNextHop("10.0.0.1")
	med := bgp.NewPathAttributeMultiExitDisc(0)
	pathAttributes := []bgp.PathAttributeInterface{origin, aspath, nexthop, med}
	nlri := bgp.NewIPAddrPrefix(24, "10.10.0.0")

	// peer1 sends normal update message 10.10.0.0/24
	update1 := bgp.NewBGPUpdateMessage(nil, pathAttributes, []*bgp.IPAddrPrefix{nlri})
	peer1 := &PeerInfo{AS: 1, Address: net.IP{1, 1, 1, 1}}
	path1 := ProcessMessage(update1, peer1, time.Now())[0]

	d := NewDestination(nlri)
	d.addNewPath(path1)
	d.Calculate(nil)

	// suppose peer2 sends grammaatically correct but semantically flawed update message
	// which has a withdrawal nlri not advertised before
	update2 := bgp.NewBGPUpdateMessage([]*bgp.IPAddrPrefix{nlri}, pathAttributes, nil)
	peer2 := &PeerInfo{AS: 2, Address: net.IP{2, 2, 2, 2}}
	path2 := ProcessMessage(update2, peer2, time.Now())[0]
	assert.Equal(t, path2.IsWithdraw, true)

	d.addWithdraw(path2)
	d.Calculate(nil)

	// we have a path from peer1 here
	assert.Equal(t, len(d.knownPathList), 1)

	// after that, new update with the same nlri comes from peer2
	update3 := bgp.NewBGPUpdateMessage(nil, pathAttributes, []*bgp.IPAddrPrefix{nlri})
	path3 := ProcessMessage(update3, peer2, time.Now())[0]
	assert.Equal(t, path3.IsWithdraw, false)

	d.addNewPath(path3)
	d.Calculate(nil)

	// this time, we have paths from peer1 and peer2
	assert.Equal(t, len(d.knownPathList), 2)

	// now peer3 sends normal update message 10.10.0.0/24
	peer3 := &PeerInfo{AS: 3, Address: net.IP{3, 3, 3, 3}}
	update4 := bgp.NewBGPUpdateMessage(nil, pathAttributes, []*bgp.IPAddrPrefix{nlri})
	path4 := ProcessMessage(update4, peer3, time.Now())[0]

	d.addNewPath(path4)
	d.Calculate(nil)

	// we must have paths from peer1, peer2 and peer3
	assert.Equal(t, len(d.knownPathList), 3)
}

func TestImplicitWithdrawCalculate(t *testing.T) {
	origin := bgp.NewPathAttributeOrigin(0)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{65001})}
	aspath := bgp.NewPathAttributeAsPath(aspathParam)
	nexthop := bgp.NewPathAttributeNextHop("10.0.0.1")
	med := bgp.NewPathAttributeMultiExitDisc(0)
	pathAttributes := []bgp.PathAttributeInterface{origin, aspath, nexthop, med}
	nlri := bgp.NewIPAddrPrefix(24, "10.10.0.101")
	updateMsg := bgp.NewBGPUpdateMessage(nil, pathAttributes, []*bgp.IPAddrPrefix{nlri})
	peer1 := &PeerInfo{AS: 1, Address: net.IP{1, 1, 1, 1}}
	path1 := ProcessMessage(updateMsg, peer1, time.Now())[0]
	path1.Filter("1", POLICY_DIRECTION_IMPORT)

	// suppose peer2 has import policy to prepend as-path
	action := &AsPathPrependAction{
		asn:    100,
		repeat: 1,
	}

	path2 := action.Apply(path1.Clone(false), nil)
	path1.Filter("2", POLICY_DIRECTION_IMPORT)
	path2.Filter("1", POLICY_DIRECTION_IMPORT)
	path2.Filter("3", POLICY_DIRECTION_IMPORT)

	d := NewDestination(nlri)
	d.addNewPath(path1)
	d.addNewPath(path2)

	d.Calculate(nil)

	assert.Equal(t, len(d.GetKnownPathList("1")), 0) // peer "1" is the originator
	assert.Equal(t, len(d.GetKnownPathList("2")), 1)
	assert.Equal(t, d.GetKnownPathList("2")[0].GetAsString(), "100 65001") // peer "2" has modified path {100, 65001}
	assert.Equal(t, len(d.GetKnownPathList("3")), 1)
	assert.Equal(t, d.GetKnownPathList("3")[0].GetAsString(), "65001") // peer "3" has original path {65001}
	assert.Equal(t, len(d.knownPathList), 2)

	// say, we removed peer2's import policy and
	// peer1 advertised new path with the same prefix
	aspathParam = []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{65001, 65002})}
	aspath = bgp.NewPathAttributeAsPath(aspathParam)
	pathAttributes = []bgp.PathAttributeInterface{origin, aspath, nexthop, med}
	updateMsg = bgp.NewBGPUpdateMessage(nil, pathAttributes, []*bgp.IPAddrPrefix{nlri})
	path3 := ProcessMessage(updateMsg, peer1, time.Now())[0]
	path3.Filter("1", POLICY_DIRECTION_IMPORT)

	d.addNewPath(path3)
	d.Calculate(nil)

	assert.Equal(t, len(d.GetKnownPathList("1")), 0) // peer "1" is the originator
	assert.Equal(t, len(d.GetKnownPathList("2")), 1)
	assert.Equal(t, d.GetKnownPathList("2")[0].GetAsString(), "65001 65002") // peer "2" has new original path {65001, 65002}
	assert.Equal(t, len(d.GetKnownPathList("3")), 1)
	assert.Equal(t, d.GetKnownPathList("3")[0].GetAsString(), "65001 65002") // peer "3" has new original path {65001, 65002}
	assert.Equal(t, len(d.knownPathList), 1)
}

func TestMedTieBreaker(t *testing.T) {
	nlri := bgp.NewIPAddrPrefix(24, "10.10.0.0")

	p0 := func() *Path {
		aspath := bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{65001, 65002}), bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{65003, 65004})})
		attrs := []bgp.PathAttributeInterface{aspath, bgp.NewPathAttributeMultiExitDisc(0)}
		return NewPath(nil, nlri, false, attrs, time.Now(), false)
	}()

	p1 := func() *Path {
		aspath := bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{65001, 65002}), bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{65003, 65004})})
		attrs := []bgp.PathAttributeInterface{aspath, bgp.NewPathAttributeMultiExitDisc(10)}
		return NewPath(nil, nlri, false, attrs, time.Now(), false)
	}()

	// same AS
	assert.Equal(t, compareByMED(p0, p1), p0)

	p2 := func() *Path {
		aspath := bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{65003})})
		attrs := []bgp.PathAttributeInterface{aspath, bgp.NewPathAttributeMultiExitDisc(10)}
		return NewPath(nil, nlri, false, attrs, time.Now(), false)
	}()

	// different AS
	assert.Equal(t, compareByMED(p0, p2), (*Path)(nil))

	p3 := func() *Path {
		aspath := bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{65001, 65002}), bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SEQ, []uint32{65003, 65004})})
		attrs := []bgp.PathAttributeInterface{aspath, bgp.NewPathAttributeMultiExitDisc(0)}
		return NewPath(nil, nlri, false, attrs, time.Now(), false)
	}()

	p4 := func() *Path {
		aspath := bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{65001, 65002}), bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SEQ, []uint32{65005, 65006})})
		attrs := []bgp.PathAttributeInterface{aspath, bgp.NewPathAttributeMultiExitDisc(10)}
		return NewPath(nil, nlri, false, attrs, time.Now(), false)
	}()

	// ignore confed
	assert.Equal(t, compareByMED(p3, p4), p3)

	p5 := func() *Path {
		attrs := []bgp.PathAttributeInterface{bgp.NewPathAttributeMultiExitDisc(0)}
		return NewPath(nil, nlri, false, attrs, time.Now(), false)
	}()

	p6 := func() *Path {
		attrs := []bgp.PathAttributeInterface{bgp.NewPathAttributeMultiExitDisc(10)}
		return NewPath(nil, nlri, false, attrs, time.Now(), false)
	}()

	// no aspath
	assert.Equal(t, compareByMED(p5, p6), p5)
}

func TestTimeTieBreaker(t *testing.T) {
	origin := bgp.NewPathAttributeOrigin(0)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{65001})}
	aspath := bgp.NewPathAttributeAsPath(aspathParam)
	nexthop := bgp.NewPathAttributeNextHop("10.0.0.1")
	med := bgp.NewPathAttributeMultiExitDisc(0)
	pathAttributes := []bgp.PathAttributeInterface{origin, aspath, nexthop, med}
	nlri := bgp.NewIPAddrPrefix(24, "10.10.0.0")
	updateMsg := bgp.NewBGPUpdateMessage(nil, pathAttributes, []*bgp.IPAddrPrefix{nlri})
	peer1 := &PeerInfo{AS: 2, LocalAS: 1, Address: net.IP{1, 1, 1, 1}, ID: net.IP{1, 1, 1, 1}}
	path1 := ProcessMessage(updateMsg, peer1, time.Now())[0]

	peer2 := &PeerInfo{AS: 2, LocalAS: 1, Address: net.IP{2, 2, 2, 2}, ID: net.IP{2, 2, 2, 2}} // weaker router-id
	path2 := ProcessMessage(updateMsg, peer2, time.Now().Add(-1*time.Hour))[0]                 // older than path1

	d := NewDestination(nlri)
	d.addNewPath(path1)
	d.addNewPath(path2)

	d.Calculate(nil)

	assert.Equal(t, len(d.knownPathList), 2)
	assert.Equal(t, true, d.GetBestPath("").GetSource().ID.Equal(net.IP{2, 2, 2, 2})) // path from peer2 win

	// this option disables tie breaking by age
	SelectionOptions.ExternalCompareRouterId = true
	d = NewDestination(nlri)
	d.addNewPath(path1)
	d.addNewPath(path2)

	d.Calculate(nil)

	assert.Equal(t, len(d.knownPathList), 2)
	assert.Equal(t, true, d.GetBestPath("").GetSource().ID.Equal(net.IP{1, 1, 1, 1})) // path from peer1 win
}

func DestCreatePeer() []*PeerInfo {
	peerD1 := &PeerInfo{AS: 65000}
	peerD2 := &PeerInfo{AS: 65001}
	peerD3 := &PeerInfo{AS: 65002}
	peerD := []*PeerInfo{peerD1, peerD2, peerD3}
	return peerD
}

func DestCreatePath(peerD []*PeerInfo) []*Path {
	bgpMsgD1 := updateMsgD1()
	bgpMsgD2 := updateMsgD2()
	bgpMsgD3 := updateMsgD3()
	pathD := make([]*Path, 3)
	for i, msg := range []*bgp.BGPMessage{bgpMsgD1, bgpMsgD2, bgpMsgD3} {
		updateMsgD := msg.Body.(*bgp.BGPUpdate)
		nlriList := updateMsgD.NLRI
		pathAttributes := updateMsgD.PathAttributes
		nlri_info := nlriList[0]
		pathD[i] = NewPath(peerD[i], nlri_info, false, pathAttributes, time.Now(), false)
	}
	return pathD
}

func updateMsgD1() *bgp.BGPMessage {

	origin := bgp.NewPathAttributeOrigin(0)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAsPathParam(2, []uint16{65000})}
	aspath := bgp.NewPathAttributeAsPath(aspathParam)
	nexthop := bgp.NewPathAttributeNextHop("192.168.50.1")
	med := bgp.NewPathAttributeMultiExitDisc(0)

	pathAttributes := []bgp.PathAttributeInterface{
		origin,
		aspath,
		nexthop,
		med,
	}

	nlri := []*bgp.IPAddrPrefix{bgp.NewIPAddrPrefix(24, "10.10.10.0")}
	updateMsg := bgp.NewBGPUpdateMessage(nil, pathAttributes, nlri)
	UpdatePathAttrs4ByteAs(updateMsg.Body.(*bgp.BGPUpdate))
	return updateMsg
}

func updateMsgD2() *bgp.BGPMessage {

	origin := bgp.NewPathAttributeOrigin(0)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAsPathParam(2, []uint16{65100})}
	aspath := bgp.NewPathAttributeAsPath(aspathParam)
	nexthop := bgp.NewPathAttributeNextHop("192.168.100.1")
	med := bgp.NewPathAttributeMultiExitDisc(100)

	pathAttributes := []bgp.PathAttributeInterface{
		origin,
		aspath,
		nexthop,
		med,
	}

	nlri := []*bgp.IPAddrPrefix{bgp.NewIPAddrPrefix(24, "20.20.20.0")}
	updateMsg := bgp.NewBGPUpdateMessage(nil, pathAttributes, nlri)
	UpdatePathAttrs4ByteAs(updateMsg.Body.(*bgp.BGPUpdate))
	return updateMsg
}
func updateMsgD3() *bgp.BGPMessage {
	origin := bgp.NewPathAttributeOrigin(0)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAsPathParam(2, []uint16{65100})}
	aspath := bgp.NewPathAttributeAsPath(aspathParam)
	nexthop := bgp.NewPathAttributeNextHop("192.168.150.1")
	med := bgp.NewPathAttributeMultiExitDisc(100)

	pathAttributes := []bgp.PathAttributeInterface{
		origin,
		aspath,
		nexthop,
		med,
	}

	nlri := []*bgp.IPAddrPrefix{bgp.NewIPAddrPrefix(24, "30.30.30.0")}
	w1 := bgp.NewIPAddrPrefix(23, "40.40.40.0")
	withdrawnRoutes := []*bgp.IPAddrPrefix{w1}
	updateMsg := bgp.NewBGPUpdateMessage(withdrawnRoutes, pathAttributes, nlri)
	UpdatePathAttrs4ByteAs(updateMsg.Body.(*bgp.BGPUpdate))
	return updateMsg
}

func TestRadixkey(t *testing.T) {
	assert.Equal(t, "000010100000001100100000", CidrToRadixkey("10.3.32.0/24"))
	assert.Equal(t, "000010100000001100100000", IpToRadixkey(net.ParseIP("10.3.32.0").To4(), 24))
	assert.Equal(t, "000010100000001100100000", IpToRadixkey(net.ParseIP("10.3.32.0").To4(), 24))
}
