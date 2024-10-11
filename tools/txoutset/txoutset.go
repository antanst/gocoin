package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/piotrnar/gocoin/lib/btc"
	"github.com/piotrnar/gocoin/lib/script"
)

func ReadVarInt(rd *bufio.Reader) (n uint64, er error) {
	var chData byte
	for {
		chData, er = rd.ReadByte()
		n = (n << 7) | uint64(chData&0x7F)
		if (chData & 0x80) != 0 {
			n++
		} else {
			return
		}
	}
}

func GetSpecialScriptSize(nSize int) int {
	if nSize == 0 || nSize == 1 {
		return 20
	}
	if nSize == 2 || nSize == 3 || nSize == 4 || nSize == 5 {
		return 32
	}
	return 0
}

func main() {
	var buf [0x400000]byte
	if len(os.Args) != 2 {
		fmt.Println("Specify the filename containing txoutset file made by bitcoin core")
		return
	}
	f, er := os.Open(os.Args[1])
	if er != nil {
		fmt.Println(er.Error())
		return
	}
	defer f.Close()

	rd := bufio.NewReaderSize(f, 16*1024*1024)

	_, er = io.ReadFull(rd, buf[:5+2+4+32+8])
	if er != nil {
		fmt.Println(er.Error())
		return
	}

	if !bytes.Equal(buf[:5], []byte("utxo\xff")) {
		fmt.Println("Bad file header", hex.EncodeToString(buf[:5]))
		return
	}
	fmt.Println("Version:", binary.LittleEndian.Uint16(buf[5:7]))
	fmt.Println("Magic bytes:", hex.EncodeToString(buf[7:11]))
	fmt.Println("Block Hash:", btc.NewUint256(buf[11:43]).String())
	coins_count := binary.LittleEndian.Uint64(buf[43:51])
	fmt.Println("Coins Count:", coins_count)

	coin_idx := uint64(0)
	var cur_txid uint64
	sha := sha256.New()

	for coin_idx != coins_count {
		if coin_idx > coins_count {
			println("coins count inconsistent", coin_idx, coins_count)
			break
		}

		var txid [32]byte
		var cnt, vout, inblock, amount, vl uint64
		var scr, compr []byte

		_, er = io.ReadFull(rd, txid[:])
		if er != nil {
			fmt.Println(er.Error())
			return
		}

		cnt, er = btc.ReadVLen(rd)
		if er != nil {
			fmt.Println(er.Error())
			return
		}

		if coin_idx+cnt > coins_count {
			println("pipa", coin_idx, cnt, coins_count)
			return
		}

		//println(cur_txid, coin_idx, btc.NewUint256(txid[:]).String(), "-", cnt, "outs")
		for _cnt := uint64(0); _cnt < cnt; _cnt++ {
			//if vout, er = ReadVarInt(rd); er != nil {
			if vout, er = btc.ReadVLen(rd); er != nil {
				fmt.Println(er.Error())
				return
			}

			inblock, er = ReadVarInt(rd)
			if er != nil {
				fmt.Println(er.Error())
				return
			}

			amount, er = ReadVarInt(rd)
			if er != nil {
				fmt.Println(er.Error())
				return
			}

			// read dummy byte
			if vl, er = ReadVarInt(rd); er != nil {
				fmt.Println(er.Error())
				return
			}

			//println(cur_txid, coin_idx, "script_length:", vl, _cnt, cnt)
			if vl < 6 {
				compr = buf[:1+GetSpecialScriptSize(int(vl))]
				compr[0] = byte(vl)
				if _, er = io.ReadFull(rd, compr[1:]); er != nil {
					fmt.Println(er.Error())
					return
				}
				scr = script.DecompressScript(compr)
			} else {
				vl -= 6
				scr = buf[:vl]
				compr = scr
				if _, er = io.ReadFull(rd, scr); er != nil {
					fmt.Println(er.Error())
					return
				}
			}

			amount = btc.DecompressAmount(amount)
			if false {
				fmt.Printf("%s-%03d  bl:%5d  cb:%d   %s BTC\n   scr:%s\n", btc.NewUint256(txid[:]).String(),
					vout, inblock/2, inblock&1, btc.UintToBtc(amount),
					hex.EncodeToString(scr))
			}

			endian := binary.LittleEndian
			sha.Write(txid[:])
			binary.Write(sha, endian, uint32(vout))
			binary.Write(sha, endian, uint32(inblock))
			binary.Write(sha, endian, uint64(amount))
			btc.WriteVlen(sha, uint64(len(scr)))
			sha.Write(scr)

			coin_idx++

			if coin_idx%1e6 == 0 {
				fmt.Println(coin_idx, "/", coins_count, ":", btc.NewUint256(txid[:]).String())
			}
		}
		cur_txid++

	}
	sum := sha.Sum(nil)
	sha.Reset()
	sha.Write(sum)
	sum = sha.Sum(nil)
	fmt.Println("Done", btc.NewUint256(sum).String())
	fmt.Println("Should be: b24ca9fc1cca9385dfc90a4d5cee603fe0e947cb4e81d06a17eafabf1fa80960")
}
