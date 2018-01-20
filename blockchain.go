package main

import (
    "time"
    "net/http"
    "fmt"
    "io/ioutil"
    "encoding/json"
    "crypto/sha256"
    "github.com/google/uuid"
    "strings"
    "flag"
    "os"
)

type Transaction struct {
    Sender string
    Recipient string
    Amount int
}

type Block struct {
    Index int
    Timestamp time.Time
    Transactions []Transaction
    Proof int
    PreviousHash string
}

type BlockChain struct {
    Chain []Block
    CurrentTransactions []Transaction
    Nodes map[string]int
}

func NewBlockChain() *BlockChain {
    b := new(BlockChain)
    b.Chain = []Block{}
    b.CurrentTransactions = []Transaction{}

    // generate genesis block
    b.NewBlock(100, "1")
    b.Nodes = map[string]int{}

    return b
}

func (b *BlockChain) NewBlock(proof int, previousHash string) Block {
    ph := previousHash

    if len(ph) == 0 {
        ph = b.hash(b.Chain[len(b.Chain)-1])
    }
    block := Block{
        Index: len(b.Chain) + 1,
        Timestamp: time.Now(),
        Transactions: b.CurrentTransactions,
        Proof: proof,
        PreviousHash: ph,
    }

    b.CurrentTransactions = []Transaction{}
    b.Chain = append(b.Chain, block)

    return block
}

func (b *BlockChain) NewTransaction(sender string, recipient string, amount int) int {
    t := Transaction{
        Sender: sender,
        Recipient: recipient,
        Amount: amount,
    }
    b.CurrentTransactions = append(b.CurrentTransactions, t)

    lb := b.LastBlock()
    return lb.Index + 1
}

func (b *BlockChain) LastBlock() Block {
    return b.Chain[len(b.Chain)-1]
}

func (b *BlockChain) ProofOfWork(lastProof int) int {
    p := 0
    for b.ValidProof(lastProof, p) == false {
        p++
    }
    return p
}

func (b *BlockChain) ValidProof(lastProof int, proof int) bool {
    guess := []byte(fmt.Sprintf("%d%d", lastProof, proof))
    guessHash := fmt.Sprintf("%x", sha256.Sum256(guess))

    return guessHash[:4] == "0000"
}

func (b *BlockChain) RegisterNode(address string) {
    _, ok := b.Nodes[address]
    if ok {
        return
    }
    b.Nodes[address] = 1
}

func (b *BlockChain) ValidChain(chain []Block) bool {
    lastBlock := chain[0]
    currentIndex := 1

    for currentIndex < len(chain) {
        block := chain[currentIndex]
        if block.PreviousHash != b.hash(lastBlock) {
            return false
        }

        if b.ValidProof(lastBlock.Proof, block.Proof) == false {
            return false
        }

        lastBlock = block
        currentIndex++
    }

    return true
}

func (b *BlockChain) ResolveConflicts() bool {
    neighbours := b.Nodes
    var newChain []Block

    maxLength := len(b.Chain)

    for node := range(neighbours) {
        address := fmt.Sprintf("%s/chain", node)
        res, err := http.Get(address)
        if err != nil {
            panic(err)
        }

        if res.StatusCode != 200 {
            continue
        }
        body, _ := ioutil.ReadAll(res.Body)
        var data struct {
            Chain []Block `json:"Chain"`
            Length int   `json:"Length"`
        }
        json.Unmarshal(body, &data)

        if data.Length > maxLength && b.ValidChain(data.Chain) {
            maxLength = data.Length
            newChain = data.Chain
        }
    }

    if len(newChain) != 0 {
        b.Chain = newChain
        return true
    }

    return false
}

func (*BlockChain) hash(block Block) string {
    data, err := json.Marshal(block)
    if err != nil {
        panic(err)
    }
    return fmt.Sprintf("%x", sha256.Sum256(data))
}

func transactionsNewHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Can't read body", http.StatusBadRequest)
        return
    }

    var jsonBody Transaction
    err = json.Unmarshal(body, &jsonBody)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        return
    }
    index := blockChain.NewTransaction(jsonBody.Sender, jsonBody.Recipient, jsonBody.Amount)
    w.WriteHeader(201)
    res := map[string]interface{}{
        "Message": fmt.Sprintf("トランザクションはブロック%dに追加されました", index),
    }

    output, err := json.Marshal(res)
    if err != nil {
        panic(err)
    }
    w.Write(output)
}

func mineHandler(w http.ResponseWriter, r *http.Request) {
    lastBlock := blockChain.LastBlock()
    lastProof := lastBlock.Proof
    proof := blockChain.ProofOfWork(lastProof)

    // 報酬
    blockChain.NewTransaction("0", nodeIdentifire, 1)

    block := blockChain.NewBlock(proof, "")

    res := map[string]interface{}{
        "Message": "新しいブロックを採掘しました",
        "Index": block.Index,
        "Transactions": block.Transactions,
        "Proof": block.Proof,
        "PreviousHash": block.PreviousHash,
    }

    output, err := json.Marshal(res)
    if err != nil {
        panic(err)
    }
    w.Write(output)
}

func chainHandler(w http.ResponseWriter, r *http.Request) {
    res := map[string]interface{}{
        "Chain": blockChain.Chain,
        "Length": len(blockChain.Chain),
    }
    output, err := json.Marshal(res)
    if err != nil {
        panic(err)
    }
    w.Write(output)
}

func nodeRegisterHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Can't read body", http.StatusBadRequest)
        return
    }

    var data map[string]interface{}
    json.Unmarshal(body, &data)
    _, ok := data["nodes"]
    if !ok {
        w.WriteHeader(500)
        return
    }
    nodes := data["nodes"].([]interface{})

    for _, node := range nodes {
        n := node.(string)
        _, ok := blockChain.Nodes[n]
        if !ok {
            blockChain.Nodes[n] = 1
        }
    }

    res := map[string]interface{}{
        "Message": "新しいノードが追加されました",
        "TotalNodes": blockChain.Nodes,
    }
    output, err := json.Marshal(res)
    if err != nil {
        panic(err)
    }
    w.Write(output)
}

func nodeResolveHandler(w http.ResponseWriter, r *http.Request) {
    replaced := blockChain.ResolveConflicts()

    var res map[string]interface{}
    if replaced {
        res = map[string]interface{}{
            "Message": "チェーンが置き換えられました",
            "NewChain": blockChain.Chain,
        }
    } else {
        res = map[string]interface{}{
            "Message": "チェーンが確認されました",
            "NewChain": blockChain.Chain,
        }
    }
    output, err := json.Marshal(res)
    if err != nil {
        panic(err)
    }
    w.Write(output)
}

var blockChain = NewBlockChain()
var nodeIdentifire = strings.Replace(uuid.New().String(), "-", "", -1)

func main() {
    var port string
    flags := flag.NewFlagSet("blockchain", flag.ContinueOnError)
    flags.StringVar(&port, "p", "5000", "port to listen on")
    flags.StringVar(&port, "port", "5000", "port to listen on")

    err := flags.Parse(os.Args[1:])
    if err != nil {
        fmt.Errorf("%v\n", err)
        return
    }

    http.HandleFunc("/transactions/new", transactionsNewHandler)
    http.HandleFunc("/mine", mineHandler)
    http.HandleFunc("/chain", chainHandler)
    http.HandleFunc("/nodes/register", nodeRegisterHandler)
    http.HandleFunc("/nodes/resolve", nodeResolveHandler)
    http.ListenAndServe(":" + port, nil)
}
