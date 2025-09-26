package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/bits"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dayanaadylkhanova/word-of-wisdom/internal/entity"
	"github.com/dayanaadylkhanova/word-of-wisdom/pkg/logger"
)

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func leadingZeroBits(b []byte) (n int) {
	for _, x := range b {
		if x == 0 {
			n += 8
			continue
		}
		return n + bits.LeadingZeros8(x)
	}
	return
}

func main() {
	log := logger.NewJSON(logger.LevelFromEnv(getenv("LOG_LEVEL", "info")))
	addr := getenv("SERVER_ADDR", "localhost:8080")
	//log.Info("starting client", "server_addr", addr)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		log.Error("dial failed", "err", err)
		os.Exit(1)
	}
	defer conn.Close()
	//log.Info("connected")

	br := bufio.NewReader(conn)
	bw := bufio.NewWriter(conn)

	// 1) challenge
	line, err := br.ReadString('\n')
	if err != nil {
		log.Error("read challenge failed", "err", err)
		os.Exit(1)
	}
	var ch entity.Challenge
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &ch); err != nil {
		log.Error("unmarshal challenge failed", "err", err)
		os.Exit(1)
	}
	//log.Info("challenge received", "difficulty", ch.Difficulty, "expires", ch.Expires)

	// соль
	salt, err := base64.StdEncoding.DecodeString(ch.SaltB64)
	if err != nil {
		log.Error("decode salt failed", "err", err)
		os.Exit(1)
	}
	expStr := strconv.FormatInt(ch.Expires, 10)

	// 2) solve: sha256( salt || ":" || expires || ":" || lower(nonce) )
	var nonce string
	for i := 0; ; i++ {
		nonce = fmt.Sprintf("%x", i)
		nonceLower := strings.ToLower(nonce)

		payload := make([]byte, 0, len(salt)+1+len(expStr)+1+len(nonceLower))
		payload = append(payload, salt...)
		payload = append(payload, ':')
		payload = append(payload, []byte(expStr)...)
		payload = append(payload, ':')
		payload = append(payload, []byte(nonceLower)...)

		sum := sha256.Sum256(payload)
		if leadingZeroBits(sum[:]) >= ch.Difficulty {
			break
		}
		if time.Now().Unix() > ch.Expires {
			log.Error("challenge expired before solved")
			os.Exit(2)
		}
		if i%500000 == 0 {
			log.Debug("searching nonce", "iter", i)
		}
	}
	//log.Info("nonce found")

	// 3) send solution
	out, _ := json.Marshal(entity.Solution{Nonce: nonce})
	if _, err := bw.Write(append(out, '\n')); err != nil {
		log.Error("write solution failed", "err", err)
		os.Exit(1)
	}
	if err := bw.Flush(); err != nil {
		log.Error("flush failed", "err", err)
		os.Exit(1)
	}

	// 4) read quote
	reply, err := br.ReadString('\n')
	if err != nil {
		log.Error("read quote failed", "err", err)
		os.Exit(1)
	}
	fmt.Println(strings.TrimSpace(reply))
	//log.Info("done")
}
