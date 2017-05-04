package webot

import (
	"github.com/skratchdot/open-golang/open"
)

// implements UUIDProcessor
type defaultUUIDProcessor struct {
	path string
}

func (dp *defaultUUIDProcessor) ProcessUUID(uuid, filepath string) error {
	// 2.``
	path, err := fetchORCodeImage(uuid,filepath)

	if err != nil {
		return err
	}
	//log.Debugf(`qrcode image path: %s`, path)

	// 3.
	go func() {
		dp.path = path
		open.Start(path)
	}()
	log.Info(`请使用微信扫一扫扫描二维码...`)

	return nil
}

func (dp *defaultUUIDProcessor) UUIDDidConfirm(err error) {
	if len(dp.path) > 0 {
		deleteFile(dp.path)
	}
}
