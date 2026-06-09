package p2p

import (
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/wire"
)

// AnnounceBlock relays an accepted local block to all currently connected peers.
// Use this after local mining/generate or submitblock so other nodes can request
// and validate the block without waiting for their periodic header sync.
func (s *Server) AnnounceBlock(hash chainhash.Hash) {
	s.announceBlockToPeers(hash)
}

// announceBlockToPeers keeps the older internal method name available and
// announces the block to every currently connected peer.
func (s *Server) announceBlockToPeers(hash chainhash.Hash) {
	s.announceBlockToPeersExcept(hash, nil)
}

func (s *Server) announceBlockToPeersExcept(hash chainhash.Hash, skip *peer) {
	payload, err := wire.InvPayload([]wire.InvVect{{Type: wire.InvTypeBlock, Hash: hash}})
	if err != nil {
		s.log.Printf("p2p: build block inv for %s: %v", hash.String(), err)
		return
	}

	sent := 0
	for _, p := range s.snapshotPeers() {
		if p == nil || p == skip {
			continue
		}
		if err := s.writePeerMessage(p, wire.CommandInv, payload); err != nil {
			s.log.Printf("p2p: announce block %s to %s: %v", hash.String(), p.remote, err)
			continue
		}
		sent++
	}
	s.addBlocksAnnounced(sent)
	s.log.Printf("p2p: announced block %s to %d peers", hash.String(), sent)
}

// AnnounceTx relays an accepted local wallet/mempool transaction to all currently connected peers.
// Peers that do not know the transaction will request it with GETDATA and validate it
// into their own mempool. This is required for wallet-created transactions to propagate
// before the next block is mined.
func (s *Server) AnnounceTx(hash chainhash.Hash) int {
	return s.announceTxToPeersExcept(hash, nil)
}

func (s *Server) announceTxToPeersExcept(hash chainhash.Hash, skip *peer) int {
	payload, err := wire.InvPayload([]wire.InvVect{{Type: wire.InvTypeTx, Hash: hash}})
	if err != nil {
		s.log.Printf("p2p: build tx inv for %s: %v", hash.String(), err)
		return 0
	}

	sent := 0
	for _, p := range s.snapshotPeers() {
		if p == nil || p == skip {
			continue
		}
		if err := s.writePeerMessage(p, wire.CommandInv, payload); err != nil {
			s.log.Printf("p2p: announce tx %s to %s: %v", hash.String(), p.remote, err)
			continue
		}
		sent++
	}
	s.log.Printf("p2p: announced tx %s to %d peers", hash.String(), sent)
	return sent
}
