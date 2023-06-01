package controlplane

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
	"github.ibm.com/mbg-agent/pkg/controlplane/eventManager"
	"github.ibm.com/mbg-agent/pkg/controlplane/healthMonitor"
	"github.ibm.com/mbg-agent/pkg/controlplane/store"
	"github.ibm.com/mbg-agent/pkg/protocol"
	httpUtils "github.ibm.com/mbg-agent/pkg/utils/http"
)

var plog = logrus.WithField("component", "mbgControlPlane/Peer")

func AddPeer(p protocol.PeerRequest) {
	//Update MBG state
	store.UpdateState()

	peerResp, err := store.GetEventManager().RaiseAddPeerEvent(eventManager.AddPeerAttr{PeerMbg: p.Id})
	if err != nil {
		plog.Errorf("Unable to raise connection request event")
		return
	}
	if peerResp.Action == eventManager.Deny {
		plog.Infof("Denying add peer(%s) due to policy", p.Id)
		return
	}
	store.AddMbgNbr(p.Id, p.Ip, p.Cport)
}

func GetAllPeers() map[string]protocol.PeerRequest {
	//Update MBG state
	store.UpdateState()
	pArr := make(map[string]protocol.PeerRequest)

	for _, s := range store.GetMbgList() {
		ip, port := store.GetMbgTargetPair(s)
		pArr[s] = protocol.PeerRequest{Id: s, Ip: ip, Cport: port}
	}
	return pArr

}

func GetPeer(peerID string) protocol.PeerRequest {
	//Update MBG state
	store.UpdateState()
	ok := store.IsMbgPeer(peerID)
	if ok {
		ip, port := store.GetMbgTargetPair(peerID)
		return protocol.PeerRequest{Id: peerID, Ip: ip, Cport: port}
	} else {
		plog.Infof("MBG %s does not exist in the peers list ", peerID)
		return protocol.PeerRequest{}
	}

}

func RemovePeer(p protocol.PeerRemoveRequest) {
	//Update MBG state
	store.UpdateState()

	err := store.GetEventManager().RaiseRemovePeerEvent(eventManager.RemovePeerAttr{PeerMbg: p.Id})
	if err != nil {
		plog.Errorf("Unable to raise connection request event")
		return
	}
	if p.Propagate {
		// Remove this MBG from the remove MBG's peer list
		peerIP := store.GetMbgTarget(p.Id)
		address := store.GetAddrStart() + peerIP + "/peer/" + store.GetMyId()
		j, err := json.Marshal(protocol.PeerRemoveRequest{Id: store.GetMyId(), Propagate: false})
		if err != nil {
			return
		}
		httpUtils.HttpDelete(address, j, store.GetHttpClient())
	}

	// Remove remote MBG from current MBG's peer
	store.RemoveMbg(p.Id)
	healthMonitor.RemoveLastSeen(p.Id)
}
