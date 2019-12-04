package filesharing

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	core "github.com/ksei/Peerster/Core"
)

const (
	DEFAULT_BUDGET   uint64 = 2
	MAX_BUDGET       uint64 = 32
	RESULT_THRESHOLD        = 2
)

//HandleSearchRequest sent from peers
func (fH *FileHandler) HandleSearchRequest(packet core.GossipPacket, sender string) {
	searchRequest := packet.SearchRequest
	if fH.isDuplicate(*searchRequest) {
		return
	}
	go fH.cacheRequest(*searchRequest)
	localMatches, found := fH.performLocalSearch(searchRequest.Keywords)
	if found {
		searchReply := &core.SearchReply{
			Origin:      fH.ctx.Name,
			Destination: searchRequest.Origin,
			HopLimit:    fH.ctx.GetHopLimit(),
			Results:     localMatches,
		}
		go fH.handleSearchReply(searchReply)
	}
	go fH.forwardSearchRequest(sender, searchRequest, searchRequest.Budget-1)
}

func (fH *FileHandler) isDuplicate(searchRequest core.SearchRequest) bool {
	fH.searchLocker.RLock()
	defer fH.searchLocker.RUnlock()
	keywords, ok := fH.requestCache[searchRequest.Origin]
	if ok && strings.Compare(strings.Join(searchRequest.Keywords, ","), keywords) == 0 {
		return true
	}
	return false
}

func (fH *FileHandler) cacheRequest(searchRequest core.SearchRequest) {
	fH.searchLocker.Lock()
	fH.requestCache[searchRequest.Origin] = strings.Join(searchRequest.Keywords, ",")
	fH.searchLocker.Unlock()
	time.Sleep(time.Duration(500) * time.Millisecond)
	fH.searchLocker.Lock()
	delete(fH.requestCache, searchRequest.Origin)
	fH.searchLocker.Unlock()
}

//LaunchSearch initiates a search from the local node
func (fH *FileHandler) LaunchSearch(keywordString *string, budgetReceived *uint64) {
	keywords := strings.Split(*keywordString, ",")
	searchRequest := &core.SearchRequest{Origin: fH.ctx.Name, Keywords: keywords, Budget: DEFAULT_BUDGET}
	fH.terminateOngoingSearchRequests <- true
	fH.searchLocker.Lock()
	fH.ongoingSearch = true
	fH.searchMatches = make(map[string]map[string][]string)
	fH.searchLocker.Unlock()
	if budgetReceived != nil {
		go fH.forwardSearchRequest(fH.ctx.Address.String(), searchRequest, *budgetReceived)
		go fH.waitToCompleteSearch()
		return
	}
	// fmt.Println("Starting Repeated Requests...")
	go fH.repeatLocalSearchRequests(searchRequest)
}

func (fH *FileHandler) waitToCompleteSearch() {
	matches := 0
	for {
		select {
		case <-fH.searchMatchFound:
			matches++
			if matches >= RESULT_THRESHOLD {
				fmt.Println("SEARCH FINISHED")
				fH.searchLocker.Lock()
				fH.ongoingSearch = false
				fH.searchLocker.Unlock()
				return
			}
		}
	}
}

func (fH *FileHandler) repeatLocalSearchRequests(searchRequest *core.SearchRequest) {
	budget := DEFAULT_BUDGET
	matches := 0
	otherSearchLaunched := false
	fH.forwardSearchRequest(fH.ctx.Address.String(), searchRequest, budget)
	for {
		select {
		case <-fH.terminateOngoingSearchRequests:
			if otherSearchLaunched {
				return
			}
			otherSearchLaunched = true
		case <-fH.searchMatchFound:
			matches++
			if matches == RESULT_THRESHOLD {
				fmt.Println("SEARCH FINISHED")
				fH.searchLocker.Lock()
				fH.ongoingSearch = false
				fH.searchLocker.Unlock()
				return
			}
		case <-time.After(1 * time.Second):
			budget = 2 * budget
			if budget > 32 {
				fmt.Println("Aborting Search: Maximum budget exhausted...")
				return
			}
			go fH.forwardSearchRequest(fH.ctx.Address.String(), searchRequest, budget)
		}
	}
}

func (fH *FileHandler) forwardSearchRequest(sender string, searchRequest *core.SearchRequest, totalBudget uint64) {
	peerList := fH.ctx.GetPeers()
	totalPeers := len(peerList)
	if int(totalBudget) < totalPeers {
		searchRequest.Budget = 1
		for _, peer := range core.RandomPeers(int(totalBudget), fH.ctx, sender) {
			go fH.ctx.SendPacketToPeer(core.GossipPacket{SearchRequest: searchRequest}, peer)
		}
	} else {
		remainingBudget := totalBudget % uint64(totalPeers)
		rand.Seed(time.Now().UnixNano())
		randomPeerIndices := rand.Perm(totalPeers)[:remainingBudget]
		for i, peer := range peerList {
			searchRequest.Budget = totalBudget / uint64(totalPeers)
			for _, randomPeer := range randomPeerIndices {
				if i == randomPeer {
					searchRequest.Budget++
					break
				}
			}
			go fH.ctx.SendPacketToPeer(core.GossipPacket{SearchRequest: searchRequest}, peer)
		}

	}
}

func (fH *FileHandler) performLocalSearch(keywords []string) ([]*core.SearchResult, bool) {
	fH.fileLocker.RLock()
	localFiles := fH.indexedFiles
	fH.fileLocker.RUnlock()
	var match bool
	results := []*core.SearchResult{}
	for _, file := range localFiles {
		match = false
		for _, keyword := range keywords {
			if strings.Index(file.Name, keyword) != -1 {
				match = true
				break
			}
		}
		if match {
			searchResult := &core.SearchResult{
				FileName:     file.Name,
				MetafileHash: file.GetMetaHash(),
				ChunkMap:     file.GetChunkMapByIndex(),
				ChunkCount:   uint64(file.GetTotalChunks()),
			}
			results = append(results, searchResult)
		}
	}

	if len(results) > 0 {
		return results, true
	}
	return nil, false
}

//HandleSearchReply manages search replies sent from peers
func (fH *FileHandler) HandleSearchReply(packet core.GossipPacket) {
	searchReply := packet.SearchReply
	go fH.handleSearchReply(searchReply)
}

func (fH *FileHandler) handleSearchReply(searchReply *core.SearchReply) {
	found, destinationIP := fH.ctx.RetrieveDestinationRoute(searchReply.Destination)
	switch found {
	case -1:
		return
	case 0:
		fH.processSearchReply(searchReply)
	default:
		if searchReply.HopLimit == 0 {
			return
		}
		searchReply.HopLimit--
		go fH.ctx.SendPacketToPeer(core.GossipPacket{SearchReply: searchReply}, destinationIP)
	}
}

func (fH *FileHandler) processSearchReply(searchReply *core.SearchReply) {

	for _, result := range searchReply.Results {
		if isMatch(result) && !fH.isRegistered(result, searchReply.Origin) {
			fH.registerSearchMatch(result, searchReply.Origin)
			fmt.Println("FOUND match", result.FileName, "at", searchReply.Origin, "metafile="+hex.EncodeToString(result.MetafileHash), "chunks="+getChunkMapString(result.ChunkMap))
			if fH.ongoingSearch {
				fH.searchMatchFound <- true
			}
			fH.ctx.GUImessageChannel <- &core.GUIPacket{SearchResult: result}
		}
	}
}

func isMatch(searchResult *core.SearchResult) bool {
	if int(searchResult.ChunkCount) == len(searchResult.ChunkMap) {
		return true
	}
	return false
}

func (fH *FileHandler) isRegistered(searchResult *core.SearchResult, origin string) bool {
	fH.searchLocker.RLock()
	defer fH.searchLocker.RUnlock()
	origins, ok := fH.searchMatches[searchResult.FileName][hex.EncodeToString(searchResult.MetafileHash)]
	if ok {
		for _, o := range origins {
			if strings.Compare(o, origin) == 0 {
				return true
			}
		}
		return false
	}
	return ok
}

func (fH *FileHandler) registerSearchMatch(searchResult *core.SearchResult, origin string) {
	fH.searchLocker.Lock()
	defer fH.searchLocker.Unlock()
	if _, ok := fH.searchMatches[searchResult.FileName]; !ok {
		fH.searchMatches[searchResult.FileName] = make(map[string][]string)
	}
	fH.searchMatches[searchResult.FileName][hex.EncodeToString(searchResult.MetafileHash)] = append(fH.searchMatches[searchResult.FileName][hex.EncodeToString(searchResult.MetafileHash)], origin)
}

func getChunkMapString(chunkMap []uint64) string {
	res := strconv.FormatUint(chunkMap[0], 10)
	for i := 1; i < len(chunkMap); i++ {
		res = res + "," + strconv.FormatUint(chunkMap[i], 10)
	}
	return res
}
