package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"io/ioutil"
	"encoding/json"
    "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/rpc/api/embedded"
)

type LocalState struct {
	PubKey string `json:"pub_key"`
	LocalPartyKey string `json:"local_party_key"`
	ParticipantKeys []string `json:"participant_keys"`
}

type Participant struct {
	PillarInfo *embedded.PillarInfo
	Keygen bool
	Online bool
	Addrs []multiaddr.Multiaddr
	ID peer.ID
}

func prettyPrint(i interface{}) string {
    s, _ := json.MarshalIndent(i, "", "\t")
    return string(s)
}

func main() {
	var participants = make(map[string]*Participant);
	var pillarInfoByProducerAddress = make(map[types.Address]*embedded.PillarInfo);
	var bootstrap = "/dns/bootstrap.zenon.community/tcp/55055/p2p/12D3KooWBVQYaz3yuJor8oW7bUqoAGDZDpFBGbGerL3SprHn57pQ";

	peerMA, err := multiaddr.NewMultiaddr(bootstrap)
	if err != nil {
		panic(err)
	}

	peerAddrInfo, err := peer.AddrInfoFromP2pAddr(peerMA)
	if err != nil {
		panic(err)
	}

	decoded, err := mh.Decode([]byte(peerAddrInfo.ID))
	if err != nil {
		panic(err)
	}

	producerAddress := types.PubKeyToAddress(decoded.Digest[4:])

	participants["*Bootstrap"] = &Participant{
		PillarInfo: &embedded.PillarInfo{
			Name: "*Bootstrap",
			BlockProducingAddress: producerAddress,
		},
		Keygen: false,
		Online: false,
		ID: peerAddrInfo.ID,
	};

	pillarInfoByProducerAddress[producerAddress] = participants["*Bootstrap"].PillarInfo;

	// TODO: connect to a go-zenon node
	rpc, err := rpc_client.NewRpcClient("ws://127.0.0.1:35998")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while trying to connect to node", err)
		return
	}

	// gets all pillars on the network
	if pillarInfoList, err := rpc.PillarApi.GetAll(0, 999); err != nil {
		fmt.Fprintf(os.Stderr, "Error while trying to call RPC", err)
	} else {
		for _, pillar := range pillarInfoList.List {
			pillarInfoByProducerAddress[pillar.BlockProducingAddress] = pillar
			participants[pillar.Name] = &Participant{
				PillarInfo: pillar,
				Keygen: false,
				Online: false,

			}
		}
	}

	// assumes a copy of the localstate-***.json file is in the same directory
	// TODO: copy the localstate.json file here or change the path
	filename := "localstate.json"
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read file: %s\n", err)
		os.Exit(1)
	}

	var localState LocalState
	if err := json.Unmarshal(data, &localState); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse JSON: %s\n", err)
		os.Exit(1)
	}


	for _, key := range localState.ParticipantKeys {
		pubBytes, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		pub, err := ic.UnmarshalEd25519PublicKey(pubBytes)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}


		producerAddress := types.PubKeyToAddress(pubBytes)

		if _, ok := participants[pillarInfoByProducerAddress[producerAddress].Name]; ok {
			participants[pillarInfoByProducerAddress[producerAddress].Name].Keygen = true
			participants[pillarInfoByProducerAddress[producerAddress].Name].ID = id
		} else {
			participants[pillarInfoByProducerAddress[producerAddress].Name] = &Participant{
				PillarInfo: pillarInfoByProducerAddress[producerAddress],
				Keygen: true,
				Online: false,
				ID: id,
			}
		}
	}

	// assumes a copy of the address_book.seed file is in the same directory
	// TODO: copy the address_book.seed file here or change the path
	content, err := ioutil.ReadFile("address_book.seed")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read file: %s\n", err)
	}

	lines := strings.Split(bootstrap + "\n" + string(content), "\n")
	host, err := libp2p.New(context.Background(), libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
    if err != nil {
        panic(err)
    }
    defer host.Close()

	// prints p2p info about current host
	// fmt.Println("Local P2P Addresses:", host.Addrs())
    // fmt.Println("Local P2P ID:", host.ID())

	for _, line := range lines {
		if len(line) < 64 {
			continue;
		}
		
		peerMA, err := multiaddr.NewMultiaddr(line)
        if err != nil {
            panic(err)
        }

        peerAddrInfo, err := peer.AddrInfoFromP2pAddr(peerMA)
        if err != nil {
            panic(err)
        }

		decoded, err := mh.Decode([]byte(peerAddrInfo.ID))
		if err != nil {
			panic(err)
		}

		pub, err := ic.UnmarshalEd25519PublicKey(decoded.Digest[4:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		producerAddress := types.PubKeyToAddress(decoded.Digest[4:])
		
		if _, ok := pillarInfoByProducerAddress[producerAddress]; !ok {
			continue
		}

		if _, ok := participants[pillarInfoByProducerAddress[producerAddress].Name]; ok {
			participants[pillarInfoByProducerAddress[producerAddress].Name].Addrs = append(participants[pillarInfoByProducerAddress[producerAddress].Name].Addrs, peerAddrInfo.Addrs...)
		} else {
			participants[pillarInfoByProducerAddress[producerAddress].Name] = &Participant{
				PillarInfo: pillarInfoByProducerAddress[producerAddress],
				Keygen: false,
				Online: false,
				ID: id,
			}
		}

		if(host.ID() == id) {
			participants[pillarInfoByProducerAddress[producerAddress].Name].Online = true;
			continue;
		}

        if err := host.Connect(context.Background(), *peerAddrInfo); err != nil {
			continue;
        }
		participants[pillarInfoByProducerAddress[producerAddress].Name].Online = true;
	}

	// prints json formatted list of all pillars in the network
	// if keygen is true, it means that pillar is a bridge participant
	// if online is true, it means that pillar has an orchestrator running
	// (keygen == true && online == true) means the pillar is a bridge participant and it is currently offline
	// (keygen == false && online == true) means the pillar is NOT a bridge participant but it is running an orchestrator
	// (keygen == false && online == false) means the pillar is NOT a bridge participant and it is NOT running an orchestrator
	fmt.Println(prettyPrint(participants))

	// prints the pillar name for all bridge participants that are currently offline 
	// for _, participant := range participants {
	// 	if(participant.Keygen == true && participant.Online == false) {
	// 		fmt.Println(prettyPrint(participant.PillarInfo.Name))
	// 	}
	// }
}
