import React, { useState, useEffect, useRef, useCallback } from 'react';
import styled from 'styled-components';
import { FaDownload, FaImage, FaFile, FaVideo } from 'react-icons/fa';
import axios from 'axios';

const ChatContainer = styled.div`
  display: flex;
  flex-direction: column;
  height: 100%;
`;

const ChatHeader = styled.div`
  padding: 15px 20px;
  background: white;
  border-bottom: 1px solid #e0e0e0;
  display: flex;
  align-items: center;
  gap: 12px;
`;

const Avatar = styled.div`
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: ${props => props.color || '#0088cc'};
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 16px;
  font-weight: 500;
`;

const ChatInfo = styled.div`
  flex: 1;
`;

const ChatTitle = styled.div`
  font-weight: 500;
  color: #333;
  margin-bottom: 4px;
`;

const ChatStatus = styled.div`
  font-size: 14px;
  color: #666;
`;

const MessagesContainer = styled.div`
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #f8f9fa;
`;

const MessageGroup = styled.div`
  margin-bottom: 20px;
`;

const DateSeparator = styled.div`
  text-align: center;
  margin: 20px 0;
  color: #666;
  font-size: 14px;
  position: relative;
  
  &::before {
    content: '';
    position: absolute;
    top: 50%;
    left: 0;
    right: 0;
    height: 1px;
    background: #e0e0e0;
    z-index: 1;
  }
  
  span {
    background: #f8f9fa;
    padding: 0 15px;
    position: relative;
    z-index: 2;
  }
`;

const Message = styled.div`
  display: flex;
  margin-bottom: 12px;
  align-items: flex-start;
  gap: 12px;
`;

const MessageAvatar = styled.div`
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #0088cc;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 12px;
  font-weight: 500;
  flex-shrink: 0;
`;

const MessageContent = styled.div`
  flex: 1;
  max-width: 70%;
`;

const MessageHeader = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
`;

const MessageSender = styled.div`
  font-weight: 500;
  color: #0088cc;
  font-size: 14px;
`;

const MessageTime = styled.div`
  font-size: 12px;
  color: #999;
`;

const MessageBubble = styled.div`
  background: white;
  border-radius: 12px;
  padding: 12px 16px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  word-wrap: break-word;
  white-space: pre-wrap;
`;

const MediaMessage = styled.div`
  background: white;
  border-radius: 12px;
  padding: 12px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  
  &:hover {
    background: #f8f9fa;
  }
`;

const MediaIcon = styled.div`
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background: #e9ecef;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666;
`;

const MediaInfo = styled.div`
  flex: 1;
`;

const MediaTitle = styled.div`
  font-weight: 500;
  color: #333;
  margin-bottom: 4px;
`;

const MediaDescription = styled.div`
  font-size: 14px;
  color: #666;
`;

const LoadingMessage = styled.div`
  text-align: center;
  padding: 20px;
  color: #666;
`;

const LoadMoreButton = styled.button`
  width: 100%;
  padding: 12px;
  background: #0088cc;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  margin-bottom: 20px;
  
  &:hover {
    background: #0077b5;
  }
  
  &:disabled {
    background: #ccc;
    cursor: not-allowed;
  }
`;

const ChatView = ({ conversation }) => {
  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [offset, setOffset] = useState(0);
  const messagesEndRef = useRef(null);
  const offsetRef = useRef(0);

  // Keep offsetRef in sync with offset state
  useEffect(() => {
    offsetRef.current = offset;
  }, [offset]);

  const loadMessages = useCallback(async (reset = false) => {
    if (!conversation) return;
    
    const currentOffset = reset ? 0 : offsetRef.current;
    
    if (reset) {
      setLoading(true);
      setMessages([]);
      setOffset(0);
      offsetRef.current = 0;
    } else {
      setLoadingMore(true);
    }

    try {
      const response = await axios.get(`/api/v1/conversations/${conversation.id}/messages`, {
        params: {
          limit: 50,
          offset: currentOffset
        }
      });

      const newMessages = response.data.messages || [];
      
      if (reset) {
        setMessages(newMessages);
        setTimeout(() => scrollToBottom(), 100);
      } else {
        setMessages(prev => [...newMessages, ...prev]);
      }

      const newOffset = currentOffset + newMessages.length;
      setOffset(newOffset);
      offsetRef.current = newOffset;
      setHasMore(newMessages.length === 50);
      
    } catch (error) {
      console.error('Failed to load messages:', error);
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }, [conversation]);

  useEffect(() => {
    if (conversation) {
      loadMessages(true);
      
      // 设置定时刷新消息 - 每30秒刷新一次
      const interval = setInterval(() => {
        loadMessages(true);
      }, 30000);
      
      return () => clearInterval(interval);
    }
  }, [conversation, loadMessages]);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  const formatDate = (timestamp) => {
    const date = new Date(timestamp);
    const now = new Date();
    const diffTime = now - date;
    const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

    if (diffDays === 1) {
      return '今天';
    } else if (diffDays === 2) {
      return '昨天';
    } else if (diffDays < 7) {
      return date.toLocaleDateString('zh-CN', { weekday: 'long' });
    } else {
      return date.toLocaleDateString('zh-CN');
    }
  };

  const formatTime = (timestamp) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('zh-CN', { 
      hour: '2-digit', 
      minute: '2-digit' 
    });
  };

  const getInitials = (name) => {
    if (!name) return '?';
    const words = name.split(' ');
    if (words.length >= 2) {
      return (words[0][0] + words[1][0]).toUpperCase();
    }
    return name.substring(0, 2).toUpperCase();
  };

  const getMediaIcon = (messageType) => {
    switch (messageType) {
      case 'photo':
        return <FaImage />;
      case 'video':
        return <FaVideo />;
      case 'document':
        return <FaFile />;
      default:
        return <FaFile />;
    }
  };

  const renderMessage = (message) => {
    const isMedia = message.message_type !== 'text';
    // 构建发送者显示名称，优先显示昵称+用户名组合
    let senderName = '';
    const fullName = message.from_first_name ? 
      `${message.from_first_name} ${message.from_last_name || ''}`.trim() : '';
    const username = message.from_username;
    
    if (fullName && username) {
      // 有昵称和用户名：显示 "昵称 (@用户名)"
      senderName = `${fullName} (@${username})`;
    } else if (fullName) {
      // 只有昵称：显示 "昵称"
      senderName = fullName;
    } else if (username) {
      // 只有用户名：显示 "@用户名"
      senderName = `@${username}`;
    } else {
      // 都没有：显示 "未知用户"
      senderName = '未知用户';
    }

    return (
      <Message key={`${message.id}-${message.message_id}`}>
        <MessageAvatar>
          {getInitials(senderName)}
        </MessageAvatar>
        <MessageContent>
          <MessageHeader>
            <MessageSender>{senderName}</MessageSender>
            <MessageTime>{formatTime(message.timestamp)}</MessageTime>
          </MessageHeader>
          
          {isMedia ? (
            <MediaMessage onClick={() => message.media_url && window.open(message.media_url)}>
              <MediaIcon>
                {getMediaIcon(message.message_type)}
              </MediaIcon>
              <MediaInfo>
                <MediaTitle>{message.content}</MediaTitle>
                <MediaDescription>{message.message_type}</MediaDescription>
              </MediaInfo>
              <FaDownload />
            </MediaMessage>
          ) : (
            <MessageBubble>
              {message.content}
            </MessageBubble>
          )}
        </MessageContent>
      </Message>
    );
  };

  const groupMessagesByDate = (messages) => {
    const groups = {};
    
    messages.forEach(message => {
      const date = new Date(message.timestamp).toDateString();
      if (!groups[date]) {
        groups[date] = [];
      }
      groups[date].push(message);
    });

    return Object.entries(groups).map(([date, messages]) => ({
      date,
      messages: messages.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp))
    }));
  };

  if (loading) {
    return (
      <ChatContainer>
        <LoadingMessage>加载消息中...</LoadingMessage>
      </ChatContainer>
    );
  }

  const messageGroups = groupMessagesByDate(messages);

  return (
    <ChatContainer>
      <ChatHeader>
        <Avatar color="#0088cc">
          {getInitials(conversation.title)}
        </Avatar>
        <ChatInfo>
          <ChatTitle>{conversation.title}</ChatTitle>
          <ChatStatus>
            {conversation.username && `@${conversation.username} • `}
            {conversation.type === 'user' && '用户'}
            {conversation.type === 'bot' && '机器人'}
            {conversation.type === 'group' && '群组'}
            {conversation.type === 'channel' && '频道'}
          </ChatStatus>
        </ChatInfo>
      </ChatHeader>

      <MessagesContainer>
        {hasMore && (
          <LoadMoreButton 
            onClick={() => loadMessages(false)}
            disabled={loadingMore}
          >
            {loadingMore ? '加载中...' : '加载更多消息'}
          </LoadMoreButton>
        )}

        {messageGroups.map(({ date, messages }) => (
          <MessageGroup key={date}>
            <DateSeparator>
              <span>{formatDate(messages[0].timestamp)}</span>
            </DateSeparator>
            {messages.map(renderMessage)}
          </MessageGroup>
        ))}
        
        <div ref={messagesEndRef} />
      </MessagesContainer>
    </ChatContainer>
  );
};

export default ChatView;