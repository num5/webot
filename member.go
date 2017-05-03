package webot

import (
	"encoding/json"
	"fmt"
)

type MemberManager struct {
	Group *User
}

func CreateMemberManagerFromGroupContact(session *Session, user *User) (*MemberManager, error) {
	b, err := WebWxBatchGetContact(session.WxWebCommon, session.WxWebXcg, session.Cookies, []*User{user})
	if err != nil {
		return nil, err
	}
	return CreateMemberManagerFromBytes(b)
}

func CreateMemberManagerFromBytes(b []byte) (*MemberManager, error) {
	var gcr GroupContactResponse
	if err := json.Unmarshal(b, &gcr); err != nil {
		return nil, err
	}
	if gcr.BaseResponse.Ret != 0 {
		return nil, fmt.Errorf("WebWxBatchGetContact ret=%d", gcr.BaseResponse.Ret)
	}

	if gcr.ContactList == nil || len(gcr.ContactList) < 1 {
		return nil, fmt.Errorf("ContactList empty")
	}

	mm := &MemberManager{
		Group: gcr.ContactList[0],
	}
	return mm, nil
}

func (s *MemberManager) Update(session *Session) error {
	members := make([]*User, len(s.Group.MemberList))
	for i, v := range s.Group.MemberList {
		members[i] = &User{
			UserName:        v.UserName,
			EncryChatRoomId: s.Group.UserName,
		}
	}
	b, err := WebWxBatchGetContact(session.WxWebCommon, session.WxWebXcg, session.Cookies, members)
	if err != nil {
		return err
	}

	var gcr GroupContactResponse
	if err := json.Unmarshal(b, &gcr); err != nil {
		return err
	}
	s.Group.MemberList = gcr.ContactList
	return nil
}

func (s *MemberManager) GetHeadImgUrlByGender(sex int) []string {
	uris := make([]string, 0)
	for _, v := range s.Group.MemberList {
		if v.Sex == sex {
			uris = append(uris, v.HeadImgUrl)
		}
	}
	return uris
}

func (s *MemberManager) GetContactsByGender(sex int) []*User {
	contacts := make([]*User, 0)
	for _, v := range s.Group.MemberList {
		if v.Sex == sex {
			contacts = append(contacts, v)
		}
	}
	return contacts
}

func (s *MemberManager) GetContactByUserName(username string) *User {
	for _, v := range s.Group.MemberList {
		if v.UserName == username {
			return v
		}
	}
	return nil
}
