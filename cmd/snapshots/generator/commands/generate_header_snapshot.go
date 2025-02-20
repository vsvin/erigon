package commands

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	kv2 "github.com/ledgerwatch/erigon/ethdb/kv"
	"github.com/spf13/cobra"

	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/common/dbutils"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/log"
)

func init() {
	withDatadir(generateHeadersSnapshotCmd)
	withSnapshotFile(generateHeadersSnapshotCmd)
	withBlock(generateHeadersSnapshotCmd)

	rootCmd.AddCommand(generateHeadersSnapshotCmd)
}

var generateHeadersSnapshotCmd = &cobra.Command{
	Use:     "headers",
	Short:   "Generate headers snapshot",
	Example: "go run cmd/snapshots/generator/main.go headers --block 11000000 --datadir /media/b00ris/nvme/snapshotsync/ --snapshotDir /media/b00ris/nvme/snapshotsync/tg/snapshots/ --snapshotMode \"hb\" --snapshot /media/b00ris/nvme/snapshots/headers_test",
	RunE: func(cmd *cobra.Command, args []string) error {
		return HeaderSnapshot(cmd.Context(), chaindata, snapshotFile, block, snapshotDir, snapshotMode)
	},
}

func HeaderSnapshot(ctx context.Context, dbPath, snapshotPath string, toBlock uint64, snapshotDir string, snapshotMode string) error {
	if snapshotPath == "" {
		return errors.New("empty snapshot path")
	}
	err := os.RemoveAll(snapshotPath)
	if err != nil {
		return err
	}
	kv := kv2.NewMDBX().Path(dbPath).MustOpen()

	snKV := kv2.NewMDBX().WithBucketsConfig(func(defaultBuckets dbutils.BucketsCfg) dbutils.BucketsCfg {
		return dbutils.BucketsCfg{
			dbutils.HeadersBucket: dbutils.BucketConfigItem{},
		}
	}).Path(snapshotPath).MustOpen()

	tx, err := kv.BeginRo(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback()
	snTx, err := snKV.BeginRw(context.Background())
	if err != nil {
		return err
	}
	defer snTx.Rollback()

	t := time.Now()
	var hash common.Hash
	var header []byte
	c, err := snTx.RwCursor(dbutils.HeadersBucket)
	if err != nil {
		return err
	}
	defer c.Close()
	for i := uint64(1); i <= toBlock; i++ {
		if common.IsCanceled(ctx) {
			return common.ErrStopped
		}

		hash, err = rawdb.ReadCanonicalHash(tx, i)
		if err != nil {
			return fmt.Errorf("getting canonical hash for block %d: %v", i, err)
		}
		header = rawdb.ReadHeaderRLP(tx, hash, i)
		if len(header) == 0 {
			return fmt.Errorf("empty header: %v", i)
		}
		if err = c.Append(dbutils.HeaderKey(i, hash), header); err != nil {
			return err
		}
		if i%1000 == 0 {
			log.Info("Committed", "block", i)
		}
	}

	err = snTx.Put(dbutils.HeadersSnapshotInfoBucket, []byte(dbutils.SnapshotHeadersHeadNumber), big.NewInt(0).SetUint64(toBlock).Bytes())
	if err != nil {
		log.Crit("SnapshotHeadersHeadNumber error", "err", err)
		return err
	}
	err = snTx.Put(dbutils.HeadersSnapshotInfoBucket, []byte(dbutils.SnapshotHeadersHeadHash), hash.Bytes())
	if err != nil {
		log.Crit("SnapshotHeadersHeadHash error", "err", err)
		return err
	}
	if err = snTx.Commit(); err != nil {
		return err
	}

	snKV.Close()

	err = os.Remove(snapshotPath + "/lock.mdb")
	if err != nil {
		log.Warn("Remove lock", "err", err)
		return err
	}
	log.Info("Finished", "duration", time.Since(t))

	return nil
}
