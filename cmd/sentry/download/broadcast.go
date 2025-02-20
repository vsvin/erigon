package download

import (
	"context"
	"math/big"

	proto_sentry "github.com/ledgerwatch/erigon-lib/gointerfaces/sentry"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/protocols/eth"
	"github.com/ledgerwatch/erigon/log"
	"github.com/ledgerwatch/erigon/rlp"
	"github.com/ledgerwatch/erigon/turbo/stages/headerdownload"
	"google.golang.org/grpc"
)

// Methods of sentry called by Core

const (
	// This is the target size for the packs of transactions or announcements. A
	// pack can get larger than this if a single transactions exceeds this size.
	maxTxPacketSize = 100 * 1024
)

func (cs *ControlServerImpl) PropagateNewBlockHashes(ctx context.Context, announces []headerdownload.Announce) {
	cs.lock.RLock()
	defer cs.lock.RUnlock()
	typedRequest := make(eth.NewBlockHashesPacket, len(announces))
	for i := range announces {
		typedRequest[i].Hash = announces[i].Hash
		typedRequest[i].Number = announces[i].Number
	}
	data, err := rlp.EncodeToBytes(&typedRequest)
	if err != nil {
		log.Error("propagateNewBlockHashes", "error", err)
		return
	}
	var req66, req65 *proto_sentry.OutboundMessageData
	for _, sentry := range cs.sentries {
		if !sentry.Ready() {
			continue
		}

		switch sentry.Protocol() {
		case eth.ETH65:
			if req65 == nil {
				req65 = &proto_sentry.OutboundMessageData{
					Id:   proto_sentry.MessageId_NEW_BLOCK_HASHES_65,
					Data: data,
				}
			}

			_, err = sentry.SendMessageToAll(ctx, req65, &grpc.EmptyCallOption{})
			if err != nil {
				log.Error("propagateNewBlockHashes", "error", err)
			}
		case eth.ETH66:
			if req66 == nil {
				req66 = &proto_sentry.OutboundMessageData{
					Id:   proto_sentry.MessageId_NEW_BLOCK_HASHES_66,
					Data: data,
				}

				_, err = sentry.SendMessageToAll(ctx, req66, &grpc.EmptyCallOption{})
				if err != nil {
					log.Error("propagateNewBlockHashes", "error", err)
				}
			}
		default:
			//??
		}
	}
}

func (cs *ControlServerImpl) BroadcastNewBlock(ctx context.Context, block *types.Block, td *big.Int) {
	cs.lock.RLock()
	defer cs.lock.RUnlock()
	data, err := rlp.EncodeToBytes(&eth.NewBlockPacket{
		Block: block,
		TD:    td,
	})
	if err != nil {
		log.Error("broadcastNewBlock", "error", err)
	}
	var req66, req65 *proto_sentry.SendMessageToRandomPeersRequest
	for _, sentry := range cs.sentries {
		if !sentry.Ready() {
			continue
		}

		switch sentry.Protocol() {
		case eth.ETH65:
			if req65 == nil {
				req65 = &proto_sentry.SendMessageToRandomPeersRequest{
					MaxPeers: 1024,
					Data: &proto_sentry.OutboundMessageData{
						Id:   proto_sentry.MessageId_NEW_BLOCK_65,
						Data: data,
					},
				}
			}

			if _, err = sentry.SendMessageToRandomPeers(ctx, req65, &grpc.EmptyCallOption{}); err != nil {
				log.Error("broadcastNewBlock", "error", err)
			}

		case eth.ETH66:
			if req66 == nil {
				req66 = &proto_sentry.SendMessageToRandomPeersRequest{
					MaxPeers: 1024,
					Data: &proto_sentry.OutboundMessageData{
						Id:   proto_sentry.MessageId_NEW_BLOCK_66,
						Data: data,
					},
				}
			}
			if _, err = sentry.SendMessageToRandomPeers(ctx, req66, &grpc.EmptyCallOption{}); err != nil {
				log.Error("broadcastNewBlock", "error", err)
			}
			continue
		}
	}
}

func (cs *ControlServerImpl) BroadcastNewTxs(ctx context.Context, txs []types.Transaction) {
	cs.lock.RLock()
	defer cs.lock.RUnlock()

	for len(txs) > 0 {
		pendingLen := maxTxPacketSize / common.HashLength
		pending := make([]common.Hash, 0, pendingLen)

		for i := 0; i < pendingLen && i < len(txs); i++ {
			pending = append(pending, txs[i].Hash())
		}
		txs = txs[len(pending):]

		data, err := rlp.EncodeToBytes(eth.NewPooledTransactionHashesPacket(pending))
		if err != nil {
			log.Error("broadcastNewBlock", "error", err)
		}
		var req66, req65 *proto_sentry.SendMessageToRandomPeersRequest
		for _, sentry := range cs.sentries {
			if !sentry.Ready() {
				continue
			}

			switch sentry.Protocol() {
			case eth.ETH65:
				if req65 == nil {
					req65 = &proto_sentry.SendMessageToRandomPeersRequest{
						MaxPeers: 1024,
						Data: &proto_sentry.OutboundMessageData{
							Id:   proto_sentry.MessageId_NEW_POOLED_TRANSACTION_HASHES_65,
							Data: data,
						},
					}
				}

				if _, err = sentry.SendMessageToRandomPeers(ctx, req65, &grpc.EmptyCallOption{}); err != nil {
					log.Error("broadcastNewBlock", "error", err)
				}

			case eth.ETH66:
				if req66 == nil {
					req66 = &proto_sentry.SendMessageToRandomPeersRequest{
						MaxPeers: 1024,
						Data: &proto_sentry.OutboundMessageData{
							Id:   proto_sentry.MessageId_NEW_POOLED_TRANSACTION_HASHES_66,
							Data: data,
						},
					}
				}
				if _, err = sentry.SendMessageToRandomPeers(ctx, req66, &grpc.EmptyCallOption{}); err != nil {
					log.Error("broadcastNewBlock", "error", err)
				}
				continue
			}
		}
	}
}
