package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
	"tgbackup/internal/api"
	"tgbackup/internal/database"
	"tgbackup/internal/telegram"
)

func main() {
	// Initialize database
	db, err := database.InitDB()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Initialize Telegram client
	tgClient := telegram.NewClient()

	// Auto-sync function with incremental updates
	autoSyncUser := func(ctx context.Context, client *telegram.Client, database *database.DB, userID int64) {
		log.Printf("Starting auto-sync for user %d", userID)
		
		// Get stored updates state
		storedPts, storedQts, storedDate, storedSeq, err := database.GetUpdatesState(userID)
		if err != nil {
			log.Printf("Failed to get updates state for user %d: %v", userID, err)
			storedPts, storedQts, storedDate, storedSeq = 0, 0, 0, 0
		}
		
		// If no stored state, do initial full sync
		if storedPts == 0 && storedQts == 0 && storedDate == 0 {
			log.Printf("No stored state found, doing initial full sync for user %d", userID)
			
			// Get dialogs from Telegram
			dialogs, err := client.GetDialogs(ctx)
			if err != nil {
				log.Printf("Auto-sync failed to get dialogs for user %d: %v", userID, err)
				return
			}

			log.Printf("Found %d dialogs for user %d", len(dialogs), userID)

			// Save conversations with user_id
			for i, dialog := range dialogs {
				dialog.UserID = userID
				log.Printf("Saving conversation %d/%d: %d (%s) - %s", i+1, len(dialogs), dialog.ID, dialog.Type, dialog.Title)
				if err := database.SaveConversation(&dialog); err != nil {
					log.Printf("Auto-sync failed to save conversation %d for user %d: %v", dialog.ID, userID, err)
				}
			}

			// Sync recent messages for each conversation
			for i, dialog := range dialogs {
				log.Printf("Auto-syncing messages %d/%d for user %d, conversation %d (%s) - %s", i+1, len(dialogs), userID, dialog.ID, dialog.Type, dialog.Title)
				
				// Add delay between requests to avoid rate limiting
				if i > 0 {
					time.Sleep(1 * time.Second)
				}
				
				messages, err := client.GetMessagesWithConvInfo(ctx, dialog.ID, 50, dialog.Type, dialog.AccessHash)
				if err != nil {
					log.Printf("Auto-sync failed to get messages for user %d, conversation %d (%s): %v", userID, dialog.ID, dialog.Title, err)
					continue
				}

				log.Printf("Auto-sync retrieved %d messages for user %d, conversation %d (%s)", len(messages), userID, dialog.ID, dialog.Title)
				for _, msg := range messages {
					msg.UserID = userID
					if err := database.SaveMessage(&msg); err != nil {
						log.Printf("Auto-sync failed to save message %d for user %d, conversation %d: %v", msg.MessageID, userID, dialog.ID, err)
					}
				}
			}
			
			// Get current state after initial sync
			state, err := client.GetState(ctx)
			if err != nil {
				log.Printf("Failed to get current state for user %d: %v", userID, err)
			} else {
				// Save the current state
				database.SaveUpdatesState(userID, state.Pts, state.Qts, state.Date, state.Seq)
				log.Printf("Saved initial state for user %d: pts=%d, qts=%d, date=%d, seq=%d", userID, state.Pts, state.Qts, state.Date, state.Seq)
			}
		} else {
			// Do incremental sync using updates
			log.Printf("Doing incremental sync for user %d with state: pts=%d, qts=%d, date=%d, seq=%d", userID, storedPts, storedQts, storedDate, storedSeq)
			
			updates, err := client.GetUpdates(ctx, storedPts, storedDate, storedQts)
			if err != nil {
				log.Printf("Failed to get updates for user %d: %v", userID, err)
				// If incremental sync fails (e.g., PERSISTENT_TIMESTAMP_EMPTY), fall back to just channel sync
				log.Printf("Falling back to channel sync only for user %d", userID)
			} else {
				// Parse and save new messages from updates
				newMessages := client.ParseUpdatesMessages(updates, userID)
				log.Printf("Found %d new messages from updates for user %d", len(newMessages), userID)
				
				for _, msg := range newMessages {
					if err := database.SaveMessage(&msg); err != nil {
						log.Printf("Failed to save new message %d for user %d: %v", msg.MessageID, userID, err)
					}
				}
				
				// Only update state if we have meaningful data
				state := updates.State
				if state.Pts > 0 || state.Date > 0 {
					database.SaveUpdatesState(userID, state.Pts, state.Qts, state.Date, state.Seq)
					log.Printf("Updated state for user %d: pts=%d, qts=%d, date=%d, seq=%d", userID, state.Pts, state.Qts, state.Date, state.Seq)
				} else {
					log.Printf("Received empty state, keeping existing state for user %d", userID)
				}
			}
			
			// Always sync channels separately as they might not appear in regular updates
			go func() {
				channels, err := database.GetConversationsByUserID(userID)
				if err != nil {
					log.Printf("Failed to get conversations for channel sync: %v", err)
					return
				}
				
				for _, conv := range channels {
					if (conv.Type == "channel" || conv.Type == "group") && conv.AccessHash != "" {
						log.Printf("Syncing %s %d (%s) for user %d", conv.Type, conv.ID, conv.Title, userID)
						
						// Get the latest message ID from database
						latestMessages, err := database.GetMessagesByUserAndConversation(userID, conv.ID, 1, 0)
						var offsetID int = 0
						if err == nil && len(latestMessages) > 0 {
							offsetID = latestMessages[0].MessageID
							log.Printf("%s %d (%s) latest message ID in DB: %d", conv.Type, conv.ID, conv.Title, offsetID)
						} else {
							log.Printf("%s %d (%s) has no messages in DB, fetching recent messages", conv.Type, conv.ID, conv.Title)
						}
						
						// Get recent messages from channel/group - fetch more messages to ensure we get new ones
						channelMessages, err := client.GetMessagesWithConvInfo(ctx, conv.ID, 50, conv.Type, conv.AccessHash)
						if err != nil {
							log.Printf("Failed to sync %s %d (%s): %v", conv.Type, conv.ID, conv.Title, err)
							continue
						}
						
						log.Printf("%s %d (%s) fetched %d messages from Telegram", conv.Type, conv.ID, conv.Title, len(channelMessages))
						
						newChannelMsgCount := 0
						for _, msg := range channelMessages {
							msg.UserID = userID
							log.Printf("%s message: ID=%d, Date=%s, Content=%.50s...", conv.Type, msg.MessageID, msg.Timestamp.Format("2006-01-02 15:04:05"), msg.Content)
							// Only save if this is a newer message or if we have no messages yet
							if offsetID == 0 || msg.MessageID > offsetID {
								if err := database.SaveMessage(&msg); err != nil {
									log.Printf("Failed to save %s message %d: %v", conv.Type, msg.MessageID, err)
								} else {
									newChannelMsgCount++
									log.Printf("Saved new %s message %d", conv.Type, msg.MessageID)
								}
							} else {
								log.Printf("Skipping old message %d (not newer than %d)", msg.MessageID, offsetID)
							}
						}
						
						if newChannelMsgCount > 0 {
							log.Printf("Successfully synced %d new messages from %s %d (%s)", newChannelMsgCount, conv.Type, conv.ID, conv.Title)
						} else {
							log.Printf("No new messages found for %s %d (%s)", conv.Type, conv.ID, conv.Title)
						}
					}
				}
			}()
		}
		
		log.Printf("Auto-sync completed for user %d", userID)
	}

	// Try to restore session on startup and auto-sync
	go func() {
		ctx := context.Background()
		session, err := db.GetActiveAuthSession()
		if err == nil && session.IsActive {
			log.Printf("Attempting to restore session on startup for App ID: %d", session.AppID)
			if err := tgClient.Connect(ctx, session.AppID, session.AppHash); err != nil {
				log.Printf("Failed to restore session on startup: %v", err)
			} else {
				log.Printf("Session restored successfully on startup")
				
				// Try to get real user info and update the default user
				time.Sleep(3 * time.Second) // Wait for connection
				if userInfo, err := tgClient.GetCurrentUserInfo(ctx); err == nil {
					log.Printf("Updating user info: %s %s", userInfo.FirstName, userInfo.LastName)
					if err := db.SaveUser(userInfo); err != nil {
						log.Printf("Failed to update user info: %v", err)
					}
					
					// Update the session with user_id
					session.UserID = userInfo.ID
					if err := db.SaveAuthSession(session); err != nil {
						log.Printf("Failed to update session with user ID: %v", err)
					} else {
						log.Printf("Updated session %d with user ID %d", session.ID, userInfo.ID)
					}
					
					// Auto-sync after startup for this user
					log.Printf("Starting initial auto-sync for user %d after startup", userInfo.ID)
					autoSyncUser(ctx, tgClient, db, userInfo.ID)
				}
			}
		}
	}()

	// Start periodic auto-sync for all active users
	go func() {
		ticker := time.NewTicker(1 * time.Minute) // 1分钟间隔
		defer ticker.Stop()

		for range ticker.C {
			ctx := context.Background()
			
			// Get all active users
			users, err := db.GetUsers()
			if err != nil {
				log.Printf("Failed to get users for periodic sync: %v", err)
				continue
			}

			for _, user := range users {
				if !user.IsActive {
					continue // Skip inactive users
				}

				// Check if we have a valid session for this user
				session, err := db.GetActiveAuthSessionByUserID(user.ID)
				if err != nil {
					log.Printf("No active session for user %d, skipping periodic sync", user.ID)
					continue
				}

				// Try to connect with user's session
				if err := tgClient.Connect(ctx, session.AppID, session.AppHash); err != nil {
					log.Printf("Failed to connect for user %d periodic sync: %v", user.ID, err)
					continue
				}

				// Check if still authenticated
				if !tgClient.IsAuthenticated(ctx) {
					log.Printf("User %d session no longer authenticated, marking inactive", user.ID)
					user.IsActive = false
					db.SaveUser(&user)
					continue
				}

				log.Printf("Starting periodic sync for user %d (%s)", user.ID, user.FirstName)
				autoSyncUser(ctx, tgClient, db, user.ID)
			}
		}
	}()

	// Initialize API handlers
	apiHandler := api.NewHandler(db, tgClient)

	// Setup Gin router
	r := gin.Default()

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})

	// API routes
	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", apiHandler.Login)
		v1.GET("/auth/status", apiHandler.GetAuthStatus)
		v1.POST("/auth/verify", apiHandler.VerifyCode)
		v1.GET("/auth/qr-status", apiHandler.CheckQRStatus)
		v1.GET("/users", apiHandler.GetUsers)
		v1.GET("/users/:id/conversations", apiHandler.GetUserConversations)
		v1.GET("/conversations", apiHandler.GetConversations)
		v1.GET("/conversations/:id/messages", apiHandler.GetMessages)
		v1.POST("/sync", apiHandler.SyncMessages)
		v1.GET("/ws", apiHandler.WebSocketHandler)
	}

	// Serve static files
	r.Static("/static", "./web/build/static")
	r.StaticFile("/", "./web/build/index.html")

	// Start server with CORS
	handler := c.Handler(r)
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}