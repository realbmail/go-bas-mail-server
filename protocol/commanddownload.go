package protocol

import (
	"bytes"
	"encoding/json"
	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"github.com/realbmail/go-bas-mail-server/bmailcrypt"
	"github.com/realbmail/go-bas-mail-server/db/bmaildb"
	"github.com/realbmail/go-bas-mail-server/db/savefile"
	"github.com/realbmail/go-bas-mail-server/tools"
	"github.com/realbmail/go-bas-mail-server/wallet"
	"github.com/realbmail/go-bmail-protocol/bmp"
	"github.com/realbmail/go-bmail-protocol/bpop"
	"github.com/realbmail/go-bmail-resolver"
	"log"
)

type CommandDownloadMsg struct {
	Sn          []byte
	CmdSyn      *bpop.CommandSyn
	CmdDownload *bpop.CmdDownload

	CmdDownAck *bpop.CmdDownloadAck
	CmdAck     *bpop.CommandAck
}

func (cdm *CommandDownloadMsg) UnPack(data []byte) error {
	cdm.CmdSyn = &bpop.CommandSyn{}
	cdm.CmdSyn.Cmd = &bpop.CmdDownload{}

	if err := json.Unmarshal(data, cdm.CmdSyn); err != nil {
		return err
	}

	cdm.CmdDownload = cdm.CmdSyn.Cmd.(*bpop.CmdDownload)

	return nil
}

func (cdm *CommandDownloadMsg) Verify() bool {
	if bytes.Compare(cdm.Sn, cdm.CmdSyn.SN[:]) != 0 {
		log.Println("sn not equals ", base58.Encode(cdm.Sn), base58.Encode(cdm.CmdSyn.SN[:]))
		return false
	}

	addr, _ := resolver.NewEthResolver(true).BMailBCA(cdm.CmdDownload.MailAddr)
	if addr != cdm.CmdDownload.Owner {
		log.Println("addr not equals", addr, cdm.CmdDownload.Owner)
		return false
	}

	if !bmailcrypt.Verify(addr.ToPubKey(), cdm.CmdSyn.SN[:], cdm.CmdSyn.Sig) {
		log.Println("verify signature failed")
		return false
	}

	return true

}

func (cdm *CommandDownloadMsg) SetCurrentSn(sn []byte) {
	cdm.Sn = sn
}

func (cdm *CommandDownloadMsg) Dispatch() error {

	return nil
}

func RecoverFromFile(eid uuid.UUID) (cep *bmp.BMailEnvelope, err error) {
	data, err := savefile.ReadFromFile(eid)

	if err != nil {
		return nil, err
	}

	cep = &bmp.BMailEnvelope{}
	err = json.Unmarshal(data, cep)
	if err != nil {
		return nil, err
	}
	return

}

func (cdm *CommandDownloadMsg) Response() (WBody, error) {

	cdm.CmdAck = &bpop.CommandAck{}
	cdm.CmdDownAck = &bpop.CmdDownloadAck{}
	cdm.CmdAck.CmdCxt = cdm.CmdDownAck

	pmdb := bmaildb.GetBMPullMailDb()

	copy(cdm.CmdAck.NextSN[:], tools.NewSn(tools.SerialNumberLength))

	sm, err := pmdb.Find(cdm.CmdDownload.MailAddr)
	if err != nil {
		cdm.CmdAck.ErrorCode = bpop.EC_No_Mail
		return cdm.CmdAck, nil
	}

	cnt := cdm.CmdDownload.MailCnt
	if cnt <= 0 {
		cnt = bpop.DefaultMailCount
	}

	total := 0

	if cdm.CmdDownload.Direction {
		for i := len(sm.Smi) - 1; i >= 0; i-- {
			if sm.Smi[i].CreateTime >= cdm.CmdDownload.TimePivot {
				continue
			}
			cep, err := RecoverFromFile(sm.Smi[i].Eid)
			if err != nil {
				continue
			}

			cdm.CmdDownAck.CryptEps = append(cdm.CmdDownAck.CryptEps, cep)
			total++
			if total >= cnt {
				break
			}
		}
	} else {
		for i := 0; i < len(sm.Smi); i++ {
			if sm.Smi[i].CreateTime <= cdm.CmdDownload.TimePivot {
				continue
			}
			cep, err := RecoverFromFile(sm.Smi[i].Eid)
			if err != nil {
				continue
			}

			cdm.CmdDownAck.CryptEps = append(cdm.CmdDownAck.CryptEps, cep)
			total++
			if total >= cnt {
				break
			}

		}
	}

	if len(cdm.CmdDownAck.CryptEps) == 0 {
		cdm.CmdAck.ErrorCode = bpop.EC_No_Mail
		return cdm.CmdAck, nil
	}

	cdm.CmdAck.Hash = cdm.CmdAck.CmdCxt.Hash()
	cdm.CmdAck.Sig = wallet.GetServerWallet().Sign(cdm.CmdAck.Hash)

	return cdm.CmdAck, nil

}
