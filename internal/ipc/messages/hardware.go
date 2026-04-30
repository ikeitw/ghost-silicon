package messages

// All hardware method names as constants so callers don't use raw strings.
const (
	MethodHardwareGetCPUCores = "hardware.getCPUCores"
	MethodHardwareGetRAM      = "hardware.getRAM"
	MethodHardwareGetGPU      = "hardware.getGPU"
	MethodNavigatorGetProfile = "navigator.getProfile"
	MethodScreenGetProfile    = "screen.getProfile"
	MethodNoiseGetCanvasSeed  = "noise.getCanvasSeed"
	MethodNoiseGetAudioSeed   = "noise.getAudioSeed"
	MethodNoiseGetWebGLSeed   = "noise.getWebGLSeed"
	MethodNoiseGetFontSeed    = "noise.getFontSeed"
	MethodStorageGetPolicy    = "storage.getPolicy"
	MethodPermissionsGetState = "permissions.getState"
	MethodSessionGetInfo      = "session.getInfo"
	MethodNetworkGetProfile   = "network.getProfile"
)
