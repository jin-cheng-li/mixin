package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ltcsuite/ltcd/btcec"
	"github.com/ltcsuite/ltcd/chaincfg"
	"github.com/ltcsuite/ltcd/chaincfg/chainhash"
	"github.com/ltcsuite/ltcd/txscript"
	"github.com/ltcsuite/ltcd/wire"
	"github.com/ltcsuite/ltcutil"
)

type UTXO struct {
	TransactionHash string
	Index           uint32
	Amount          int64
	PrivateKey      string
}

func LocalNormalizePublicKey(address string) (string, error) {
	address = strings.TrimSpace(address)
	ltcAddress, err := ltcutil.DecodeAddress(address, &chaincfg.MainNetParams)
	if err != nil {
		return "", err
	}
	if ltcAddress.String() != address {
		return "", fmt.Errorf("Litecoin NormalizeAddress mismatch %s", address)
	}
	return ltcAddress.String(), nil
}

func LocalEstimateTransactionFee(inputs []*UTXO, feePerKb int64) int64 {
	estimatedRawSizeInKb := int64(len(inputs))*160/1024 + 1
	return feePerKb * estimatedRawSizeInKb
}

func LocalSignRawTransaction(inputs []*UTXO, output string, amount int64, feePerKb int64, changeAddress string) (string, string, int64, error) {
	tx, inputAmount := wire.NewMsgTx(wire.TxVersion), int64(0)

	for _, input := range inputs {
		hash, err := chainhash.NewHashFromStr(input.TransactionHash)
		if err != nil {
			return "", "", 0, err
		}
		txIn := &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  *hash,
				Index: input.Index,
			},
			Sequence: 0xffffffff,
		}
		tx.AddTxIn(txIn)
		inputAmount = inputAmount + input.Amount
	}

	addressPubKeyHash, err := ltcutil.DecodeAddress(output, &chaincfg.MainNetParams)
	if err != nil {
		return "", "", 0, err
	}
	pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
	if err != nil {
		return "", "", 0, err
	}
	tx.AddTxOut(wire.NewTxOut(amount, pkScript))

	estimatedRawSizeInKb := int64(len(inputs))*160/1024 + 1
	feeToConsumed := feePerKb * estimatedRawSizeInKb
	changeAmount := inputAmount - feeToConsumed - amount
	if changeAmount < 0 {
		return "", "", 0, fmt.Errorf("insuficcient trasaction fee %d %d %d", inputAmount, feePerKb, estimatedRawSizeInKb)
	}
	if changeAmount > feePerKb {
		addressPubKeyHash, err := ltcutil.DecodeAddress(changeAddress, &chaincfg.MainNetParams)
		if err != nil {
			return "", "", 0, err
		}
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return "", "", 0, err
		}
		tx.AddTxOut(wire.NewTxOut(changeAmount, pkScript))
	} else {
		feeToConsumed = inputAmount - amount
	}

	for idx, input := range inputs {
		privateKeyBytes, err := hex.DecodeString(input.PrivateKey)
		if err != nil {
			return "", "", 0, err
		}
		privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), privateKeyBytes)
		if err != nil {
			return "", "", 0, err
		}
		addressPubKey, err := ltcutil.NewAddressPubKey(publicKey.SerializeCompressed(), &chaincfg.MainNetParams)
		if err != nil {
			return "", "", 0, err
		}
		addressPubKeyHash := addressPubKey.AddressPubKeyHash()
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return "", "", 0, err
		}
		sigScript, err := txscript.SignatureScript(tx, idx, pkScript, txscript.SigHashAll, privateKey, true)
		if err != nil {
			return "", "", 0, err
		}
		tx.TxIn[idx].SignatureScript = sigScript
	}

	var rawBuffer bytes.Buffer
	err = tx.BtcEncode(&rawBuffer, wire.ProtocolVersion, wire.BaseEncoding)
	if err != nil {
		return "", "", 0, err
	}
	rawBytes := rawBuffer.Bytes()
	if rawSizeInKb := int64(len(rawBytes))/1024 + 1; rawSizeInKb > estimatedRawSizeInKb {
		return "", "", 0, fmt.Errorf("raw size estimation error %d %d", rawSizeInKb, estimatedRawSizeInKb)
	}
	if estimatedRawSizeInKb > 100 {
		return "", "", 0, fmt.Errorf("Litecoin transaction size too large %d", estimatedRawSizeInKb)
	}
	return tx.TxHash().String(), hex.EncodeToString(rawBytes), feeToConsumed, nil
}

func LocalGenerateKey() (string, string, error) {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	if err != nil {
		return "", "", err
	}
	privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), seed)
	addressPubKey, err := ltcutil.NewAddressPubKey(publicKey.SerializeCompressed(), &chaincfg.MainNetParams)
	if err != nil {
		return "", "", err
	}
	private := hex.EncodeToString(privateKey.Serialize())
	public := addressPubKey.AddressPubKeyHash().EncodeAddress()
	return public, private, nil
}
