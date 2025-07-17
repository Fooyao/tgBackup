package telegram

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"tgbackup/internal/models"
)

type Client struct {
	client      *telegram.Client
	api         *tg.Client
	isConnected bool
	authFlow    *auth.Flow
	appID       int
	appHash     string
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewClient() *Client {
	return &Client{
		isConnected: false,
	}
}

func (c *Client) Connect(ctx context.Context, appID int, appHash string) error {
	// If already connected to the same app, return success
	if c.isConnected && c.appID == appID && c.appHash == appHash {
		return nil
	}
	
	// Close existing connection if any
	if c.cancel != nil {
		c.cancel()
		c.isConnected = false
	}
	
	logger, _ := zap.NewDevelopment()
	
	c.appID = appID
	c.appHash = appHash
	
	// Create cancellable context for the client
	c.ctx, c.cancel = context.WithCancel(ctx)
	
	// Create session storage directory
	sessionDir := "./sessions"
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %v", err)
	}
	
	// Create session storage
	sessionFile := filepath.Join(sessionDir, fmt.Sprintf("session_%d.json", appID))
	sessionStorage := &session.FileStorage{
		Path: sessionFile,
	}
	
	options := telegram.Options{
		Logger:         logger,
		SessionStorage: sessionStorage,
	}

	client := telegram.NewClient(appID, appHash, options)
	c.client = client

	// Start client in background
	go func() {
		err := client.Run(c.ctx, func(ctx context.Context) error {
			c.api = client.API()
			c.isConnected = true
			// Keep running until context is cancelled
			<-ctx.Done()
			c.isConnected = false
			return nil
		})
		if err != nil {
			logger.Error("Client error", zap.Error(err))
			c.isConnected = false
		}
	}()

	// Wait a bit for connection to establish
	time.Sleep(2 * time.Second)
	
	return nil
}

type codeAuth struct {
	phone string
}

func (a codeAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a codeAuth) Password(_ context.Context) (string, error) {
	return "", fmt.Errorf("password not supported")
}

func (a codeAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	return "", fmt.Errorf("code should be provided externally")
}

func (a codeAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error {
	return nil
}

func (c *Client) StartAuth(ctx context.Context, phone string) (string, error) {
	if !c.isConnected {
		return "", fmt.Errorf("client not connected")
	}

	// Wait for client to be ready
	for i := 0; i < 10; i++ {
		if c.api != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if c.api == nil {
		return "", fmt.Errorf("telegram client not ready")
	}

	// Send code
	sentCode, err := c.api.AuthSendCode(ctx, &tg.AuthSendCodeRequest{
		PhoneNumber: phone,
		APIID:       c.appID,
		APIHash:     c.appHash,
		Settings: tg.CodeSettings{
			AllowFlashcall:  false,
			CurrentNumber:   false,
			AllowAppHash:    false,
			AllowMissedCall: false,
			AllowFirebase:   false,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send code: %v", err)
	}

	// Extract PhoneCodeHash from the response
	switch s := sentCode.(type) {
	case *tg.AuthSentCode:
		return s.PhoneCodeHash, nil
	default:
		return "", fmt.Errorf("unexpected response type: %T", sentCode)
	}
}

func (c *Client) VerifyCode(ctx context.Context, phone, code, codeHash string) error {
	if !c.isConnected {
		return fmt.Errorf("client not connected")
	}

	_, err := c.api.AuthSignIn(ctx, &tg.AuthSignInRequest{
		PhoneNumber:   phone,
		PhoneCodeHash: codeHash,
		PhoneCode:     code,
	})
	return err
}

func (c *Client) IsAuthenticated(ctx context.Context) bool {
	if !c.isConnected {
		return false
	}

	_, err := c.api.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	return err == nil
}

func (c *Client) GetCurrentUserInfo(ctx context.Context) (*models.User, error) {
	if !c.isConnected {
		return nil, fmt.Errorf("client not connected")
	}

	if c.api == nil {
		return nil, fmt.Errorf("telegram client not ready")
	}

	fullUser, err := c.api.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	if err != nil {
		return nil, fmt.Errorf("failed to get current user info: %v", err)
	}

	// Extract user info from the response
	user, ok := fullUser.Users[0].(*tg.User)
	if !ok {
		return nil, fmt.Errorf("unexpected user type in response")
	}

	return &models.User{
		ID:           user.ID,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Username:     user.Username,
		Phone:        user.Phone,
		IsActive:     true, // If we can fetch info, session is active
		LastSyncTime: time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func (c *Client) GetDialogs(ctx context.Context) ([]models.Conversation, error) {
	if !c.isConnected {
		return nil, fmt.Errorf("client not connected")
	}

	// Wait for client to be ready
	for i := 0; i < 10; i++ {
		if c.api != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if c.api == nil {
		return nil, fmt.Errorf("telegram client not ready")
	}

	dialogs, err := c.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetDate: 0,
		OffsetID:   0,
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
		Hash:       0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dialogs: %v", err)
	}

	var conversations []models.Conversation

	switch d := dialogs.(type) {
	case *tg.MessagesDialogs:
		for _, dialog := range d.Dialogs {
			conv := c.parseDialog(dialog, d.Chats, d.Users)
			if conv.Title != "" { // Only add if we parsed it successfully
				conversations = append(conversations, conv)
			}
		}
	case *tg.MessagesDialogsSlice:
		for _, dialog := range d.Dialogs {
			conv := c.parseDialog(dialog, d.Chats, d.Users)
			if conv.Title != "" { // Only add if we parsed it successfully
				conversations = append(conversations, conv)
			}
		}
	}

	return conversations, nil
}

func (c *Client) parseDialog(dialog tg.DialogClass, chats []tg.ChatClass, users []tg.UserClass) models.Conversation {
	d, ok := dialog.(*tg.Dialog)
	if !ok {
		return models.Conversation{} // Return empty if not a dialog
	}

	peer := d.Peer

	var conv models.Conversation
	var title, username string
	var avatarURL string

	switch p := peer.(type) {
	case *tg.PeerUser:
		for _, u := range users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				firstName := user.FirstName
				lastName := user.LastName
				if firstName == "" && lastName == "" {
					title = fmt.Sprintf("User %d", user.ID)
				} else {
					title = fmt.Sprintf("%s %s", firstName, lastName)
				}
				title = strings.TrimSpace(title)
				username = user.Username
				conv.Type = "user"
				if user.Bot {
					conv.Type = "bot"
				}
				conv.ID = user.ID
				// Store access_hash for user
				conv.AccessHash = fmt.Sprintf("%d", user.AccessHash)
				// Get avatar URL from user.Photo
				if user.Photo != nil {
					avatarURL = c.getUserPhotoURL(user.Photo)
				}
				break
			}
		}
	case *tg.PeerChat:
		for _, ch := range chats {
			if chat, ok := ch.(*tg.Chat); ok && chat.ID == p.ChatID {
				title = chat.Title
				if title == "" {
					title = fmt.Sprintf("Chat %d", chat.ID)
				}
				conv.Type = "group"
				conv.ID = chat.ID
				// Chat doesn't have access_hash, use empty string
				conv.AccessHash = ""
				// Get avatar URL from chat.Photo
				if chat.Photo != nil {
					avatarURL = c.getChatPhotoURL(chat.Photo)
				}
				break
			}
		}
	case *tg.PeerChannel:
		for _, ch := range chats {
			if channel, ok := ch.(*tg.Channel); ok && channel.ID == p.ChannelID {
				title = channel.Title
				if title == "" {
					title = fmt.Sprintf("Channel %d", channel.ID)
				}
				username = channel.Username
				conv.Type = "channel"
				if channel.Broadcast {
					conv.Type = "channel"
				} else {
					conv.Type = "group" // Supergroup
				}
				conv.ID = channel.ID
				// Store access_hash for channel
				conv.AccessHash = fmt.Sprintf("%d", channel.AccessHash)
				// Get avatar URL from channel.Photo
				if channel.Photo != nil {
					avatarURL = c.getChatPhotoURL(channel.Photo)
				}
				break
			}
		}
	}

	conv.Title = title
	conv.Username = username
	conv.AvatarURL = avatarURL
	conv.LastTime = time.Now()

	return conv
}

func (c *Client) GetMessages(ctx context.Context, peerID int64, limit int) ([]models.Message, error) {
	if !c.isConnected {
		return nil, fmt.Errorf("client not connected")
	}

	// This is a simplified version that tries different peer types
	// In the sync process, we should use the proper method with conversation info
	return c.getMessagesWithFallback(ctx, peerID, limit, "", "")
}

func (c *Client) GetMessagesWithConvInfo(ctx context.Context, peerID int64, limit int, convType, accessHash string) ([]models.Message, error) {
	if !c.isConnected {
		return nil, fmt.Errorf("client not connected")
	}

	var peer tg.InputPeerClass
	var err error

	// Use the correct peer type based on conversation info
	switch convType {
	case "user", "bot":
		if accessHash != "" {
			if hash, parseErr := strconv.ParseInt(accessHash, 10, 64); parseErr == nil {
				peer = &tg.InputPeerUser{UserID: peerID, AccessHash: hash}
			} else {
				peer = &tg.InputPeerUser{UserID: peerID}
			}
		} else {
			peer = &tg.InputPeerUser{UserID: peerID}
		}
	case "channel":
		if accessHash != "" {
			if hash, parseErr := strconv.ParseInt(accessHash, 10, 64); parseErr == nil {
				peer = &tg.InputPeerChannel{ChannelID: peerID, AccessHash: hash}
			} else {
				return nil, fmt.Errorf("channel requires valid access_hash")
			}
		} else {
			return nil, fmt.Errorf("channel requires access_hash")
		}
	case "group":
		// If group has access_hash, it's actually a supergroup (uses channel-style access)
		if accessHash != "" {
			if hash, parseErr := strconv.ParseInt(accessHash, 10, 64); parseErr == nil {
				peer = &tg.InputPeerChannel{ChannelID: peerID, AccessHash: hash}
			} else {
				return nil, fmt.Errorf("supergroup requires valid access_hash")
			}
		} else {
			// Regular group/chat
			peer = &tg.InputPeerChat{ChatID: peerID}
		}
	default:
		return c.getMessagesWithFallback(ctx, peerID, limit, convType, accessHash)
	}

	messages, err := c.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: limit,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get messages for peer %d (type: %s): %v", peerID, convType, err)
	}

	return c.parseMessagesResponse(messages, peerID)
}

func (c *Client) getMessagesWithFallback(ctx context.Context, peerID int64, limit int, convType, accessHash string) ([]models.Message, error) {
	var messages tg.MessagesMessagesClass
	var err error
	
	// Try as user first (includes bots)
	peer := &tg.InputPeerUser{UserID: peerID}
	messages, err = c.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: limit,
	})
	
	// If user peer fails, try as channel
	if err != nil {
		peer := &tg.InputPeerChannel{ChannelID: peerID}
		messages, err = c.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  peer,
			Limit: limit,
		})
	}
	
	// If channel peer fails, try as chat
	if err != nil {
		peer := &tg.InputPeerChat{ChatID: peerID}
		messages, err = c.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  peer,
			Limit: limit,
		})
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to get messages for peer %d: %v", peerID, err)
	}

	return c.parseMessagesResponse(messages, peerID)
}

func (c *Client) parseMessagesResponse(messages tg.MessagesMessagesClass, peerID int64) ([]models.Message, error) {
	var result []models.Message

	switch m := messages.(type) {
	case *tg.MessagesMessages:
		for _, msg := range m.Messages {
			if message, ok := msg.(*tg.Message); ok {
				parsedMsg := c.parseMessageWithUsers(message, m.Users)
				parsedMsg.ConversationID = peerID
				result = append(result, parsedMsg)
			}
		}
	case *tg.MessagesMessagesSlice:
		for _, msg := range m.Messages {
			if message, ok := msg.(*tg.Message); ok {
				parsedMsg := c.parseMessageWithUsers(message, m.Users)
				parsedMsg.ConversationID = peerID
				result = append(result, parsedMsg)
			}
		}
	case *tg.MessagesChannelMessages:
		for _, msg := range m.Messages {
			if message, ok := msg.(*tg.Message); ok {
				parsedMsg := c.parseMessageWithUsers(message, m.Users)
				parsedMsg.ConversationID = peerID
				result = append(result, parsedMsg)
			}
		}
	}

	return result, nil
}

func (c *Client) parseMessageWithUsers(msg *tg.Message, users []tg.UserClass) models.Message {
	var content string
	var messageType string = "text"
	var mediaURL string
	var mediaName string
	var fromUsername string
	var fromFirstName string
	var fromLastName string

	content = msg.Message

	// Handle media messages
	if msg.Media != nil {
		switch media := msg.Media.(type) {
		case *tg.MessageMediaPhoto:
			messageType = "photo"
			if content == "" {
				content = "[Photo]"
			}
			// Extract photo URL
			if photo, ok := media.Photo.(*tg.Photo); ok {
				mediaURL = c.getPhotoURL(photo)
			}
		case *tg.MessageMediaDocument:
			if doc, ok := media.Document.(*tg.Document); ok {
				// Check if it's a video, audio, or other document
				for _, attr := range doc.Attributes {
					switch a := attr.(type) {
					case *tg.DocumentAttributeVideo:
						messageType = "video"
						if content == "" {
							content = "[Video]"
						}
					case *tg.DocumentAttributeAudio:
						messageType = "audio"
						if content == "" {
							content = "[Audio]"
						}
						if a.Title != "" {
							mediaName = a.Title
						}
					case *tg.DocumentAttributeFilename:
						mediaName = a.FileName
					case *tg.DocumentAttributeImageSize:
						if messageType == "text" {
							messageType = "image"
							if content == "" {
								content = "[Image]"
							}
						}
					case *tg.DocumentAttributeAnimated:
						messageType = "gif"
						if content == "" {
							content = "[GIF]"
						}
					case *tg.DocumentAttributeSticker:
						messageType = "sticker"
						if content == "" {
							content = "[Sticker]"
						}
					}
				}
				
				// Default to document if no specific type found
				if messageType == "text" {
					messageType = "document"
					if content == "" {
						content = "[Document]"
					}
				}
				
				// Set media name if not already set
				if mediaName == "" {
					mediaName = fmt.Sprintf("%s.%s", messageType, "file")
				}
				
				// Get document URL
				mediaURL = c.getDocumentURL(doc)
			}
		case *tg.MessageMediaWebPage:
			// Keep as text message but note it has a webpage
			if content == "" {
				content = "[Webpage]"
			}
		case *tg.MessageMediaContact:
			messageType = "contact"
			if content == "" {
				content = "[Contact]"
			}
		case *tg.MessageMediaGeo:
			messageType = "location"
			if content == "" {
				content = "[Location]"
			}
		case *tg.MessageMediaPoll:
			messageType = "poll"
			if content == "" {
				content = "[Poll]"
			}
		}
	}

	// Handle FromID - it can be nil
	var fromID int64
	if msg.FromID != nil {
		switch from := msg.FromID.(type) {
		case *tg.PeerUser:
			fromID = from.UserID
			// Find user information from users list
			for _, u := range users {
				if user, ok := u.(*tg.User); ok && user.ID == from.UserID {
					fromUsername = user.Username
					fromFirstName = user.FirstName
					fromLastName = user.LastName
					break
				}
			}
		case *tg.PeerChat:
			fromID = from.ChatID
		case *tg.PeerChannel:
			fromID = from.ChannelID
		}
	}

	// Add media name to content if available
	if mediaName != "" && content != "" {
		content = fmt.Sprintf("%s\n📁 %s", content, mediaName)
	}

	return models.Message{
		MessageID:       msg.ID,
		FromID:          fromID,
		FromUsername:    fromUsername,
		FromFirstName:   fromFirstName,
		FromLastName:    fromLastName,
		Content:         content,
		MessageType:     messageType,
		MediaURL:        mediaURL,
		Timestamp:       time.Unix(int64(msg.Date), 0),
	}
}

func (c *Client) parseMessage(msg *tg.Message) models.Message {
	var content string
	var messageType string = "text"
	var mediaURL string
	var mediaName string

	content = msg.Message

	// Handle media messages
	if msg.Media != nil {
		switch media := msg.Media.(type) {
		case *tg.MessageMediaPhoto:
			messageType = "photo"
			if content == "" {
				content = "[Photo]"
			}
			// Extract photo URL
			if photo, ok := media.Photo.(*tg.Photo); ok {
				mediaURL = c.getPhotoURL(photo)
			}
		case *tg.MessageMediaDocument:
			if doc, ok := media.Document.(*tg.Document); ok {
				// Check if it's a video, audio, or other document
				for _, attr := range doc.Attributes {
					switch a := attr.(type) {
					case *tg.DocumentAttributeVideo:
						messageType = "video"
						if content == "" {
							content = "[Video]"
						}
					case *tg.DocumentAttributeAudio:
						messageType = "audio"
						if content == "" {
							content = "[Audio]"
						}
						if a.Title != "" {
							mediaName = a.Title
						}
					case *tg.DocumentAttributeFilename:
						mediaName = a.FileName
					case *tg.DocumentAttributeImageSize:
						if messageType == "text" {
							messageType = "image"
							if content == "" {
								content = "[Image]"
							}
						}
					case *tg.DocumentAttributeAnimated:
						messageType = "gif"
						if content == "" {
							content = "[GIF]"
						}
					case *tg.DocumentAttributeSticker:
						messageType = "sticker"
						if content == "" {
							content = "[Sticker]"
						}
					}
				}
				
				// Default to document if no specific type found
				if messageType == "text" {
					messageType = "document"
					if content == "" {
						content = "[Document]"
					}
				}
				
				// Set media name if not already set
				if mediaName == "" {
					mediaName = fmt.Sprintf("%s.%s", messageType, "file")
				}
				
				// Get document URL
				mediaURL = c.getDocumentURL(doc)
			}
		case *tg.MessageMediaWebPage:
			// Keep as text message but note it has a webpage
			if content == "" {
				content = "[Webpage]"
			}
		case *tg.MessageMediaContact:
			messageType = "contact"
			if content == "" {
				content = "[Contact]"
			}
		case *tg.MessageMediaGeo:
			messageType = "location"
			if content == "" {
				content = "[Location]"
			}
		case *tg.MessageMediaPoll:
			messageType = "poll"
			if content == "" {
				content = "[Poll]"
			}
		}
	}

	// Handle FromID - it can be nil
	var fromID int64
	if msg.FromID != nil {
		switch from := msg.FromID.(type) {
		case *tg.PeerUser:
			fromID = from.UserID
		case *tg.PeerChat:
			fromID = from.ChatID
		case *tg.PeerChannel:
			fromID = from.ChannelID
		}
	}

	// Add media name to content if available
	if mediaName != "" && content != "" {
		content = fmt.Sprintf("%s\n📁 %s", content, mediaName)
	}

	return models.Message{
		MessageID:   msg.ID,
		FromID:      fromID,
		Content:     content,
		MessageType: messageType,
		MediaURL:    mediaURL,
		Timestamp:   time.Unix(int64(msg.Date), 0),
	}
}

func (c *Client) GenerateQRCode(ctx context.Context) (string, error) {
	if !c.isConnected {
		return "", fmt.Errorf("client not connected")
	}

	// Wait for client to be ready
	for i := 0; i < 10; i++ {
		if c.api != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if c.api == nil {
		return "", fmt.Errorf("telegram client not ready")
	}

	// Export QR login token
	qrLogin, err := c.api.AuthExportLoginToken(ctx, &tg.AuthExportLoginTokenRequest{
		APIID:     c.appID,
		APIHash:   c.appHash,
		ExceptIDs: []int64{},
	})
	if err != nil {
		return "", fmt.Errorf("failed to export login token: %v", err)
	}

	switch token := qrLogin.(type) {
	case *tg.AuthLoginToken:
		// Convert token to base64 for QR code
		tokenBase64 := base64.URLEncoding.EncodeToString(token.Token)
		qrURL := fmt.Sprintf("tg://login?token=%s", tokenBase64)
		return qrURL, nil
	default:
		return "", fmt.Errorf("unexpected QR login response type: %T", qrLogin)
	}
}

func (c *Client) CheckQRCode(ctx context.Context, token []byte) error {
	if !c.isConnected {
		return fmt.Errorf("client not connected")
	}

	// Wait for client to be ready
	for i := 0; i < 10; i++ {
		if c.api != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if c.api == nil {
		return fmt.Errorf("telegram client not ready")
	}

	// Import QR login token
	auth, err := c.api.AuthImportLoginToken(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to import login token: %v", err)
	}

	switch a := auth.(type) {
	case *tg.AuthLoginTokenSuccess:
		return nil
	case *tg.AuthLoginTokenMigrateTo:
		return fmt.Errorf("need to migrate to DC %d", a.DCID)
	default:
		return fmt.Errorf("unexpected auth response: %T", auth)
	}
}

func (c *Client) getPhotoURL(photo *tg.Photo) string {
	if len(photo.Sizes) == 0 {
		return ""
	}
	
	// Find the largest photo size
	var largestSize *tg.PhotoSize
	var maxSize int
	
	for _, size := range photo.Sizes {
		if photoSize, ok := size.(*tg.PhotoSize); ok {
			currentSize := photoSize.W * photoSize.H
			if currentSize > maxSize {
				maxSize = currentSize
				largestSize = photoSize
			}
		}
	}
	
	if largestSize != nil {
		// In a real implementation, you would use the file location to generate a proper URL
		// For now, we'll create a placeholder that includes the file info
		return fmt.Sprintf("telegram://photo/%d_%d", photo.ID, largestSize.Size)
	}
	
	return ""
}

func (c *Client) getDocumentURL(doc *tg.Document) string {
	if doc == nil {
		return ""
	}
	
	// In a real implementation, you would use the document info to generate a proper URL
	// For now, we'll create a placeholder that includes the document info
	return fmt.Sprintf("telegram://document/%d_%d", doc.ID, doc.Size)
}

func (c *Client) getUserPhotoURL(userPhoto tg.UserProfilePhotoClass) string {
	if userPhoto == nil {
		return ""
	}
	
	if photo, ok := userPhoto.(*tg.UserProfilePhoto); ok {
		return fmt.Sprintf("telegram://avatar/%d", photo.PhotoID)
	}
	
	return ""
}

func (c *Client) getChatPhotoURL(chatPhoto tg.ChatPhotoClass) string {
	if chatPhoto == nil {
		return ""
	}
	
	if photo, ok := chatPhoto.(*tg.ChatPhoto); ok {
		return fmt.Sprintf("telegram://chat_avatar/%d", photo.PhotoID)
	}
	
	return ""
}

// GetUpdates gets new messages using Telegram's updates.getDifference API
func (c *Client) GetUpdates(ctx context.Context, pts int, date int, qts int) (*tg.UpdatesDifference, error) {
	if !c.isConnected {
		return nil, fmt.Errorf("client not connected")
	}

	if c.api == nil {
		return nil, fmt.Errorf("telegram client not ready")
	}

	// Get difference (incremental updates)
	updates, err := c.api.UpdatesGetDifference(ctx, &tg.UpdatesGetDifferenceRequest{
		Pts:  pts,
		Date: date,
		Qts:  qts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get updates: %v", err)
	}

	switch u := updates.(type) {
	case *tg.UpdatesDifference:
		return u, nil
	case *tg.UpdatesDifferenceSlice:
		// Convert slice to regular difference
		return &tg.UpdatesDifference{
			NewMessages:          u.NewMessages,
			NewEncryptedMessages: u.NewEncryptedMessages,
			OtherUpdates:         u.OtherUpdates,
			Chats:               u.Chats,
			Users:               u.Users,
			State:               u.IntermediateState,
		}, nil
	case *tg.UpdatesDifferenceEmpty:
		// No new updates, but return empty difference with date to preserve state
		return &tg.UpdatesDifference{
			State: tg.UpdatesState{
				Pts:  0,
				Qts:  0,
				Date: u.Date,
				Seq:  0,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unexpected updates type: %T", updates)
	}
}

// GetState gets current updates state
func (c *Client) GetState(ctx context.Context) (*tg.UpdatesState, error) {
	if !c.isConnected {
		return nil, fmt.Errorf("client not connected")
	}

	if c.api == nil {
		return nil, fmt.Errorf("telegram client not ready")
	}

	state, err := c.api.UpdatesGetState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %v", err)
	}

	return state, nil
}

// ParseUpdatesMessages converts updates to our message format
func (c *Client) ParseUpdatesMessages(updates *tg.UpdatesDifference, userID int64) []models.Message {
	var messages []models.Message

	// Parse new messages from updates
	for _, msg := range updates.NewMessages {
		if message, ok := msg.(*tg.Message); ok {
			parsedMsg := c.parseMessageWithUsers(message, updates.Users)
			parsedMsg.UserID = userID
			
			// Determine conversation ID from the message peer
			if message.PeerID != nil {
				switch peer := message.PeerID.(type) {
				case *tg.PeerUser:
					parsedMsg.ConversationID = peer.UserID
				case *tg.PeerChat:
					parsedMsg.ConversationID = peer.ChatID
				case *tg.PeerChannel:
					parsedMsg.ConversationID = peer.ChannelID
				}
			}
			
			messages = append(messages, parsedMsg)
		}
	}

	return messages
}

// GetChannelMessages gets messages from a specific channel
func (c *Client) GetChannelMessages(ctx context.Context, channelID int64, accessHash string, limit int, offsetID int) ([]models.Message, error) {
	if !c.isConnected {
		return nil, fmt.Errorf("client not connected")
	}

	if c.api == nil {
		return nil, fmt.Errorf("telegram client not ready")
	}

	// Parse access hash
	hash, err := strconv.ParseInt(accessHash, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid access hash: %v", err)
	}

	peer := &tg.InputPeerChannel{
		ChannelID:  channelID,
		AccessHash: hash,
	}

	messages, err := c.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:     peer,
		Limit:    limit,
		OffsetID: offsetID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get channel messages: %v", err)
	}

	return c.parseMessagesResponse(messages, channelID)
}

func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.isConnected = false
	return nil
}