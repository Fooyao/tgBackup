package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"tgbackup/internal/database"
	"tgbackup/internal/models"
	"tgbackup/internal/telegram"
)

type Handler struct {
	db       *database.DB
	tgClient *telegram.Client
	upgrader websocket.Upgrader
}

func NewHandler(db *database.DB, tgClient *telegram.Client) *Handler {
	return &Handler{
		db:       db,
		tgClient: tgClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

type LoginRequest struct {
	Phone    string `json:"phone"`
	UseQR    bool   `json:"use_qr"`
}

type LoginResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	QRCode      string `json:"qr_code,omitempty"`
	PhoneHash   string `json:"phone_hash,omitempty"`
	RequireCode bool   `json:"require_code"`
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// Use default App ID and App Hash
	const defaultAppID = 24133254
	const defaultAppHash = "cf33b107b32979433261506f1c586867"

	// Connect to Telegram
	if err := h.tgClient.Connect(ctx, defaultAppID, defaultAppHash); err != nil {
		log.Printf("Failed to connect to Telegram: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to Telegram"})
		return
	}

	// Wait a bit more for connection to stabilize
	time.Sleep(3 * time.Second)

	var response LoginResponse

	if req.UseQR {
		// QR Code login
		qrCode, err := h.tgClient.GenerateQRCode(ctx)
		if err != nil {
			log.Printf("QR code generation error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate QR code: %v", err)})
			return
		}

		// Save QR session info
		session := &models.AuthSession{
			IsActive:    false,
			AppID:       defaultAppID,
			AppHash:     defaultAppHash,
			SessionData: qrCode, // Store QR code for later verification
		}
		if err := h.db.SaveAuthSession(session); err != nil {
			log.Printf("Failed to save QR auth session: %v", err)
		}

		response = LoginResponse{
			Success: true,
			Message: "QR code generated. Please scan with Telegram app",
			QRCode:  qrCode,
		}
	} else {
		// Phone number login
		phoneHash, err := h.tgClient.StartAuth(ctx, req.Phone)
		if err != nil {
			log.Printf("Phone auth error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start auth: %v", err)})
			return
		}

		// Save auth session
		session := &models.AuthSession{
			PhoneCode: phoneHash,
			IsActive:  false, // Not active until verification is complete
			AppID:     defaultAppID,
			AppHash:   defaultAppHash,
			Phone:     req.Phone,
		}
		if err := h.db.SaveAuthSession(session); err != nil {
			log.Printf("Failed to save auth session: %v", err)
		}

		response = LoginResponse{
			Success:     true,
			Message:     "Verification code sent",
			PhoneHash:   phoneHash,
			RequireCode: true,
		}
	}

	c.JSON(http.StatusOK, response)
}

type VerifyRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

func (h *Handler) VerifyCode(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// Get pending auth session (not necessarily active)
	session, err := h.db.GetPendingAuthSession()
	if err != nil {
		log.Printf("No pending auth session found: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active auth session"})
		return
	}

	// Verify code
	if err := h.tgClient.VerifyCode(ctx, req.Phone, req.Code, session.PhoneCode); err != nil {
		log.Printf("Code verification failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid verification code"})
		return
	}

	// Get current user info and save to database
	userInfo, err := h.tgClient.GetCurrentUserInfo(ctx)
	if err != nil {
		log.Printf("Failed to get user info after auth: %v", err)
	} else {
		// Save user to database
		if err := h.db.SaveUser(userInfo); err != nil {
			log.Printf("Failed to save user info: %v", err)
		}
		// Update session with user ID
		session.UserID = userInfo.ID
	}

	// Update session to active
	session.IsActive = true
	if err := h.db.SaveAuthSession(session); err != nil {
		log.Printf("Failed to update auth session: %v", err)
	}

	// Start automatic sync in background after successful login
	if userInfo != nil {
		go func() {
			time.Sleep(2 * time.Second) // Give client time to establish connection
			h.autoSyncAfterLogin(ctx, userInfo.ID)
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Authentication successful",
	})
}

func (h *Handler) GetAuthStatus(c *gin.Context) {
	ctx := context.Background()
	
	// First check if client is authenticated
	isAuth := h.tgClient.IsAuthenticated(ctx)
	var userInfo map[string]interface{}
	
	// If not authenticated, try to restore from saved session
	if !isAuth {
		session, err := h.db.GetActiveAuthSession()
		if err == nil && session.IsActive {
			log.Printf("Attempting to restore session for App ID: %d", session.AppID)
			// Try to reconnect using saved session
			if err := h.tgClient.Connect(ctx, session.AppID, session.AppHash); err == nil {
				// Wait a bit for connection and check again
				time.Sleep(3 * time.Second)
				isAuth = h.tgClient.IsAuthenticated(ctx)
				if isAuth {
					log.Printf("Session successfully restored")
					// Get current user info from Telegram
					if info, err := h.tgClient.GetCurrentUserInfo(ctx); err == nil {
						userInfo = map[string]interface{}{
							"id":         info.ID,
							"first_name": info.FirstName,
							"last_name":  info.LastName,
							"username":   info.Username,
							"phone":      info.Phone,
						}
					}
				} else {
					log.Printf("Session restoration failed - client not authenticated")
				}
			} else {
				log.Printf("Failed to reconnect with saved session: %v", err)
			}
		} else {
			log.Printf("No active session found in database")
		}
	} else {
		log.Printf("Client already authenticated")
		// Get current user info from Telegram
		if info, err := h.tgClient.GetCurrentUserInfo(ctx); err == nil {
			userInfo = map[string]interface{}{
				"id":         info.ID,
				"first_name": info.FirstName,
				"last_name":  info.LastName,
				"username":   info.Username,
				"phone":      info.Phone,
			}
		}
	}

	response := gin.H{
		"authenticated": isAuth,
	}
	if userInfo != nil {
		response["user"] = userInfo
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) CheckQRStatus(c *gin.Context) {
	ctx := context.Background()
	
	// Check if already authenticated (session restored)
	isAuth := h.tgClient.IsAuthenticated(ctx)
	
	if isAuth {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Already authenticated",
			"authenticated": true,
		})
		return
	}
	
	// Get the most recent QR session
	session, err := h.db.GetPendingAuthSession()
	if err != nil || session.SessionData == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No QR session found"})
		return
	}

	// If we have a QR session but not authenticated, check QR status
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "QR code not scanned yet",
		"authenticated": false,
	})
}

func (h *Handler) GetConversations(c *gin.Context) {
	conversations, err := h.db.GetConversations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get conversations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversations": conversations,
	})
}

func (h *Handler) GetMessages(c *gin.Context) {
	idStr := c.Param("id")
	conversationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation ID"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	offsetStr := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	messages, err := h.db.GetMessages(conversationID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
	})
}

func (h *Handler) SyncMessages(c *gin.Context) {
	ctx := context.Background()

	// Check if authenticated, if not try to restore session
	if !h.tgClient.IsAuthenticated(ctx) {
		log.Printf("Client not authenticated, attempting to restore session")
		
		// Try to restore from saved session
		session, err := h.db.GetActiveAuthSession()
		if err != nil {
			log.Printf("No active session found for sync: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}
		
		// Reconnect using saved session
		if err := h.tgClient.Connect(ctx, session.AppID, session.AppHash); err != nil {
			log.Printf("Failed to reconnect for sync: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to restore session"})
			return
		}
		
		// Wait for connection and check again
		time.Sleep(3 * time.Second)
		if !h.tgClient.IsAuthenticated(ctx) {
			log.Printf("Session restoration failed for sync")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session restoration failed"})
			return
		}
		
		log.Printf("Session successfully restored for sync")
	}

	// Get dialogs from Telegram
	dialogs, err := h.tgClient.GetDialogs(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get dialogs"})
		return
	}

	// Get current user ID for data association
	var currentUserID int64
	if userInfo, err := h.tgClient.GetCurrentUserInfo(ctx); err == nil {
		currentUserID = userInfo.ID
		// Update user sync time
		userInfo.LastSyncTime = time.Now()
		h.db.SaveUser(userInfo)
	}

	// Save conversations with user_id
	for _, dialog := range dialogs {
		dialog.UserID = currentUserID
		if err := h.db.SaveConversation(&dialog); err != nil {
			log.Printf("Failed to save conversation %d: %v", dialog.ID, err)
		}
	}

	// Sync messages for each conversation
	go func() {
		for _, dialog := range dialogs {
			log.Printf("Syncing messages for conversation %d (%s) - %s", dialog.ID, dialog.Type, dialog.Title)
			messages, err := h.tgClient.GetMessagesWithConvInfo(ctx, dialog.ID, 100, dialog.Type, dialog.AccessHash)
			if err != nil {
				log.Printf("Failed to get messages for conversation %d (%s): %v", dialog.ID, dialog.Title, err)
				continue
			}

			log.Printf("Retrieved %d messages for conversation %d (%s)", len(messages), dialog.ID, dialog.Title)
			for _, msg := range messages {
				msg.UserID = currentUserID
				if err := h.db.SaveMessage(&msg); err != nil {
					log.Printf("Failed to save message %d in conversation %d: %v", msg.MessageID, dialog.ID, err)
				}
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Sync started",
		"conversations": len(dialogs),
	})
}

func (h *Handler) GetUsers(c *gin.Context) {
	users, err := h.db.GetUsers()
	if err != nil {
		log.Printf("Failed to get users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
	})
}

func (h *Handler) GetUserConversations(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	conversations, err := h.db.GetConversationsByUserID(userID)
	if err != nil {
		log.Printf("Failed to get conversations for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get conversations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversations": conversations,
	})
}

func (h *Handler) WebSocketHandler(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}
	defer conn.Close()

	// Handle WebSocket messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		// Handle different message types
		switch msg["type"] {
		case "ping":
			conn.WriteJSON(map[string]interface{}{
				"type": "pong",
			})
		case "sync_status":
			// Send sync status
			conn.WriteJSON(map[string]interface{}{
				"type":    "sync_status",
				"running": false, // TODO: Implement actual sync status
			})
		}
	}
}

func (h *Handler) autoSyncAfterLogin(ctx context.Context, userID int64) {
	log.Printf("Starting automatic sync for user %d after login", userID)
	
	// Check if authenticated
	if !h.tgClient.IsAuthenticated(ctx) {
		log.Printf("Client not authenticated for auto-sync, skipping")
		return
	}

	// Get dialogs from Telegram
	dialogs, err := h.tgClient.GetDialogs(ctx)
	if err != nil {
		log.Printf("Auto-sync failed to get dialogs: %v", err)
		return
	}

	// Save conversations with user_id
	for _, dialog := range dialogs {
		dialog.UserID = userID
		if err := h.db.SaveConversation(&dialog); err != nil {
			log.Printf("Auto-sync failed to save conversation %d: %v", dialog.ID, err)
		}
	}

	// Sync messages for each conversation
	go func() {
		for _, dialog := range dialogs {
			log.Printf("Auto-syncing messages for conversation %d (%s) - %s", dialog.ID, dialog.Type, dialog.Title)
			messages, err := h.tgClient.GetMessagesWithConvInfo(ctx, dialog.ID, 50, dialog.Type, dialog.AccessHash)
			if err != nil {
				log.Printf("Auto-sync failed to get messages for conversation %d (%s): %v", dialog.ID, dialog.Title, err)
				continue
			}

			log.Printf("Auto-sync retrieved %d messages for conversation %d (%s)", len(messages), dialog.ID, dialog.Title)
			for _, msg := range messages {
				msg.UserID = userID
				if err := h.db.SaveMessage(&msg); err != nil {
					log.Printf("Auto-sync failed to save message %d in conversation %d: %v", msg.MessageID, dialog.ID, err)
				}
			}
		}
		log.Printf("Auto-sync completed for user %d", userID)
	}()
}