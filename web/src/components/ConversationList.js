import React from 'react';
import styled from 'styled-components';

const ListContainer = styled.div`
  flex: 1;
  overflow-y: auto;
`;

const ConversationItem = styled.div`
  padding: 15px 20px;
  border-bottom: 1px solid #e0e0e0;
  cursor: pointer;
  background: ${props => props.selected ? '#e3f2fd' : 'white'};
  transition: background-color 0.2s ease;
  
  &:hover {
    background: ${props => props.selected ? '#e3f2fd' : '#f5f5f5'};
  }
`;

const ConversationHeader = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
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

const ConversationInfo = styled.div`
  flex: 1;
  min-width: 0;
`;

const ConversationTitle = styled.div`
  font-weight: 500;
  color: #333;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const ConversationUsername = styled.div`
  font-size: 14px;
  color: #666;
  display: flex;
  align-items: center;
  gap: 6px;
`;

const ConversationMeta = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 8px;
`;

const LastMessage = styled.div`
  font-size: 14px;
  color: #666;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  margin-right: 10px;
`;

const LastTime = styled.div`
  font-size: 12px;
  color: #999;
  white-space: nowrap;
`;

const EmptyState = styled.div`
  padding: 40px 20px;
  text-align: center;
  color: #666;
`;

const ConversationList = ({ 
  conversations, 
  selectedConversation, 
  onSelectConversation,
  getConversationIcon 
}) => {
  const getAvatarColor = (type) => {
    switch (type) {
      case 'user':
        return '#28a745';
      case 'bot':
        return '#17a2b8';
      case 'group':
        return '#fd7e14';
      case 'channel':
        return '#6f42c1';
      default:
        return '#0088cc';
    }
  };

  const formatTime = (timestamp) => {
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now - date;
    const oneDay = 24 * 60 * 60 * 1000;

    if (diff < oneDay) {
      return date.toLocaleTimeString('zh-CN', { 
        hour: '2-digit', 
        minute: '2-digit' 
      });
    } else if (diff < 7 * oneDay) {
      return date.toLocaleDateString('zh-CN', { 
        weekday: 'short' 
      });
    } else {
      return date.toLocaleDateString('zh-CN', { 
        month: 'short', 
        day: 'numeric' 
      });
    }
  };

  const getInitials = (title) => {
    if (!title) return '?';
    const words = title.split(' ');
    if (words.length >= 2) {
      return (words[0][0] + words[1][0]).toUpperCase();
    }
    return title.substring(0, 2).toUpperCase();
  };

  if (!conversations || conversations.length === 0) {
    return (
      <ListContainer>
        <EmptyState>
          暂无会话记录
          <br />
          请点击"同步"按钮获取Telegram会话
        </EmptyState>
      </ListContainer>
    );
  }

  return (
    <ListContainer>
      {conversations.map((conversation) => (
        <ConversationItem
          key={conversation.id}
          selected={selectedConversation?.id === conversation.id}
          onClick={() => onSelectConversation(conversation)}
        >
          <ConversationHeader>
            <Avatar color={getAvatarColor(conversation.type)}>
              {conversation.avatar_url && !conversation.avatar_url.startsWith('telegram://') ? (
                <img 
                  src={conversation.avatar_url} 
                  alt={conversation.title}
                  style={{ 
                    width: '100%', 
                    height: '100%', 
                    borderRadius: '50%',
                    objectFit: 'cover' 
                  }}
                  onError={(e) => {
                    e.target.style.display = 'none';
                    e.target.parentNode.innerHTML = getInitials(conversation.title);
                  }}
                />
              ) : (
                getInitials(conversation.title)
              )}
            </Avatar>
            <ConversationInfo>
              <ConversationTitle>{conversation.title}</ConversationTitle>
              <ConversationUsername>
                {getConversationIcon(conversation.type)}
                {conversation.username && `@${conversation.username}`}
              </ConversationUsername>
            </ConversationInfo>
          </ConversationHeader>
          
          <ConversationMeta>
            <LastMessage>
              {conversation.last_message || '暂无消息'}
            </LastMessage>
            <LastTime>
              {conversation.last_time && formatTime(conversation.last_time)}
            </LastTime>
          </ConversationMeta>
        </ConversationItem>
      ))}
    </ListContainer>
  );
};

export default ConversationList;