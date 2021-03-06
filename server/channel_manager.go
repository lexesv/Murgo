package server

import (
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"

	"murgo/pkg/mumbleproto"
	"murgo/pkg/servermodule"
	APIkeys "murgo/server/util"
	"reflect"
)

const ROOT_CHANNEL = 0

type ChannelManager struct {

	//todo add numChannel and keep channel ids
	channelList   map[int]*Channel
	nextChannelID int
	rootChannel   *Channel
}

func (c *ChannelManager) Init() {
	servermodule.RegisterAPI((*ChannelManager).SendChannelList, APIkeys.SendChannelList)
	servermodule.RegisterAPI((*ChannelManager).EnterChannel, APIkeys.EnterChannel)
	servermodule.RegisterAPI((*ChannelManager).BroadCastChannel, APIkeys.BroadcastChannel)
	servermodule.RegisterAPI((*ChannelManager).AddChannel, APIkeys.AddChannel)
	//assign heap

	c.init()

}
func (c *ChannelManager) init() {
	c.channelList = make(map[int]*Channel)

	// set root channel as default channel for all user
	rootChannel := NewChannel(ROOT_CHANNEL, "RootChannel")
	c.rootChannel = rootChannel
	c.channelList[ROOT_CHANNEL] = rootChannel

	//Add one for each channel ID
	c.nextChannelID = ROOT_CHANNEL + 1
}

func (channelManager *ChannelManager) AddChannel(channelName string, client *Client) {
	for _, eachChannel := range channelManager.channelList {
		if eachChannel.Name == channelName {
			//todo : client object 없에는 과정
			//sendPermissionDenied(client, mumbleproto.PermissionDenied_ChannelName)
			fmt.Println("duplicated channel name")
			return
		}
	}
	// create new channel
	channel := NewChannel(channelManager.nextChannelID, channelName)
	channelManager.nextChannelID++
	channelManager.channelList[channel.Id] = channel

	//let all session know the created channel
	channelStateMsg := channel.toChannelState()

	servermodule.Cast(APIkeys.BroadcastMessage, channelStateMsg)
	//let the channel creator enter the channel
	channelManager.EnterChannel(channel.Id, client)

}

func (channelManager *ChannelManager) RootChannel() *Channel {
	return channelManager.channelList[ROOT_CHANNEL]
}

func (channelManager *ChannelManager) exitChannel(client *Client, channel *Channel) {

}

//broadcast a msg to all users in a channel

func (channelManager *ChannelManager) BroadCastChannel(channelId int, msg interface{}) {
	channel, err := channelManager.channel(channelId)
	if err != nil {
		fmt.Println(err)
	}
	for _, client := range channel.clients {
		client.SendMessage(msg)
	}
}

func (channelManager *ChannelManager) broadCastChannelWithoutMe(channelId int, msg interface{}, client *Client) {
	channel, err := channelManager.channel(channelId)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(channel.clients)
	for _, eachClient := range channel.clients {
		if reflect.DeepEqual(client, eachClient) {
			continue
		}
		eachClient.SendMessage(msg)
	}
}

func (channelManager *ChannelManager) channel(channelId int) (*Channel, error) {
	if channel, ok := channelManager.channelList[channelId]; ok {
		return channel, nil
	}

	return nil, errors.New("Channel ID in invalid in channel list")
}

func (channelManager *ChannelManager) SendChannelList(client *Client) {
	fmt.Println(len(channelManager.channelList))
	for _, eachChannel := range channelManager.channelList {

		client.SendMessage(eachChannel.toChannelState())
	}
}

func (channelManager *ChannelManager) EnterChannel(channelId int, client *Client) {

	newChannel, err := channelManager.channel(channelId)
	//fmt.Println(client.UserName, " will enter ", newChannel.Name)
	if err != nil {
		panic("Channel Id doesn't exist")
	}
	oldChannel := client.Channel

	if oldChannel == newChannel {
		return
	}

	if oldChannel != nil {
		oldChannel.removeClient(client)
		if oldChannel.IsEmpty() {
			channelManager.removeChannel(oldChannel)
			oldChannel = nil
		}
	}

	client.Channel = newChannel
	newChannel.addClient(client)
	userState := client.ToUserState()

	if oldChannel != nil && oldChannel.Id != ROOT_CHANNEL {
		//이전 채널에 떠났음을 알림
		channelManager.broadCastChannelWithoutMe(oldChannel.Id, userState, client)
	}
	// 변한 상태를 클라이언트에게 알림

	if newChannel.Id != ROOT_CHANNEL {
		//새 채널입장을 채널 유저들에게 알림
		channelManager.broadCastChannelWithoutMe(newChannel.Id, userState, client)
		//채널에 있는 유저들을 입장하는 유저에게 알림
		newChannel.SendUserListInChannel(client)
		/*	for _, users := range newChannel.clients {
			client.sendMessage(users.ToUserState())

		}*/
		//client.SendMessage(userState)
	} else {
		client.SendMessage(userState)
	}

	//for test
	for _, eachChannel := range channelManager.channelList {
		fmt.Print(eachChannel.Name, ": ")
		for _, eachUser := range eachChannel.clients {
			fmt.Print(eachUser.UserName, ", ")
		}
		fmt.Println()
	}
}

func (channelManager *ChannelManager) removeChannel(tempChannel interface{}) {
	channel := tempChannel.(*Channel)
	// Can't remove root
	if channel.Id == ROOT_CHANNEL {
		return
	}

	// Remove all clients in the channel to root
	for _, client := range channel.clients {

		userStateMsg := &mumbleproto.UserState{}
		userStateMsg.Session = proto.Uint32(client.Session())
		userStateMsg.ChannelId = proto.Uint32(uint32(ROOT_CHANNEL))
		channelManager.EnterChannel(ROOT_CHANNEL, client)

		//channelManager.Call(channelManager.supervisor.sessionManager)
		servermodule.Cast(APIkeys.BroadcastMessage, userStateMsg)
	}

	// Remove the channel itself
	rootChannel, err := channelManager.channel(ROOT_CHANNEL)
	if err != nil {
		panic("Root doesn't exist")
	}
	delete(channelManager.channelList, channel.Id)
	delete(rootChannel.children, channel.Id)

	channelRemoveMsg := &mumbleproto.ChannelRemove{
		ChannelId: proto.Uint32(uint32(channel.Id)),
	}
	servermodule.Cast(APIkeys.BroadcastMessage, channelRemoveMsg)
}

func (channelManager *ChannelManager) printChannels() {
	fmt.Println("channel list : ")
	for _, channel := range channelManager.channelList {
		fmt.Print(channel.Name, ", ")
	}
	fmt.Println()
}
