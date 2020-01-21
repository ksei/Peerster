package SecretSharing

import (
	core "github.com/ksei/Peerster/Core"
)

func (ssHandler *SSHandler) isDuplicate(passwordUID string) bool {
	ssHandler.ssLocker.RLock()
	defer ssHandler.ssLocker.RUnlock()

	_, exists := ssHandler.requestedPasswordStatus[passwordUID]
	return exists
}

func (ssHandler *SSHandler) initiateShareCollection(passwordUID string) {
	shareRequest := &core.ShareRequest{
		Origin:     ssHandler.ctx.Name,
		Budget:     124,
		RequestUID: passwordUID,
	}

	ssHandler.registerPasswordRequest(passwordUID)

}
