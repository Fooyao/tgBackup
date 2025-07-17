import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import styled from 'styled-components';
import { FaSync, FaSignOutAlt, FaUser, FaRobot, FaUsers, FaBroadcastTower } from 'react-icons/fa';
import { useAuth } from '../hooks/useAuth';
import ConversationList from '../components/ConversationList';
import ChatView from '../components/ChatView';
import axios from 'axios';

const MainContainer = styled.div`
  display: flex;
  height: 100vh;
  background: white;
`;

const Sidebar = styled.div`
  width: 350px;
  background: #f8f9fa;
  border-right: 1px solid #e0e0e0;
  display: flex;
  flex-direction: column;
`;

const SidebarHeader = styled.div`
  padding: 20px;
  border-bottom: 1px solid #e0e0e0;
  background: white;
`;

const Title = styled.h2`
  color: #333;
  margin-bottom: 15px;
  display: flex;
  align-items: center;
  gap: 10px;
`;

const ButtonGroup = styled.div`
  display: flex;
  gap: 10px;
`;

const ActionButton = styled.button`
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.3s ease;
  
  &.sync {
    background: #28a745;
    color: white;
    
    &:hover {
      background: #218838;
    }
  }
  
  &.logout {
    background: #dc3545;
    color: white;
    
    &:hover {
      background: #c82333;
    }
  }
  
  &:disabled {
    background: #ccc;
    cursor: not-allowed;
  }
`;

const ChatContainer = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
`;

const EmptyState = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666;
  font-size: 18px;
`;

const StatusBar = styled.div`
  padding: 10px 20px;
  background: #e9ecef;
  border-top: 1px solid #dee2e6;
  font-size: 14px;
  color: #666;
`;

const ConfirmModal = styled.div`
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
`;

const ConfirmDialog = styled.div`
  background: white;
  border-radius: 8px;
  padding: 24px;
  max-width: 400px;
  width: 90%;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
`;

const ConfirmTitle = styled.h3`
  margin: 0 0 16px 0;
  color: #dc3545;
  font-size: 18px;
`;

const ConfirmText = styled.p`
  margin: 0 0 16px 0;
  color: #666;
  line-height: 1.5;
`;

const ConfirmInput = styled.input`
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
  margin-bottom: 16px;
  box-sizing: border-box;
`;

const ConfirmButtons = styled.div`
  display: flex;
  gap: 12px;
  justify-content: flex-end;
`;

const ConfirmButton = styled.button`
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  
  &.cancel {
    background: #6c757d;
    color: white;
    
    &:hover {
      background: #545b62;
    }
  }
  
  &.confirm {
    background: #dc3545;
    color: white;
    
    &:hover {
      background: #c82333;
    }
    
    &:disabled {
      background: #ccc;
      cursor: not-allowed;
    }
  }
`;

const UserList = styled.div`
  padding: 15px 20px;
  border-bottom: 1px solid #e0e0e0;
  background: white;
  max-height: 200px;
  overflow-y: auto;
`;

const UserItem = styled.div`
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  margin-bottom: 6px;
  background: ${props => props.selected ? '#e3f2fd' : 'transparent'};
  border: ${props => props.selected ? '2px solid #2196f3' : '1px solid #e0e0e0'};
  transition: all 0.2s ease;
  
  &:hover {
    background: #f5f5f5;
  }
  
  &:last-child {
    margin-bottom: 0;
  }
`;

const UserName = styled.div`
  font-weight: 600;
  color: #333;
  font-size: 14px;
`;

const UserStatus = styled.div`
  font-size: 12px;
  color: ${props => props.active ? '#4caf50' : '#999'};
  margin-top: 2px;
`;

const UserListTitle = styled.h4`
  margin: 0 0 10px 0;
  color: #666;
  font-size: 14px;
  font-weight: 500;
`;

const MainPage = () => {
  const [users, setUsers] = useState([]);
  const [selectedUser, setSelectedUser] = useState(null);
  const [conversations, setConversations] = useState([]);
  const [selectedConversation, setSelectedConversation] = useState(null);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false);
  const [confirmText, setConfirmText] = useState('');
  const [syncStatus, setSyncStatus] = useState({
    totalConversations: 0,
    totalMessages: 0,
    lastSyncTime: null
  });

  const { isAuthenticated, logout } = useAuth();
  const navigate = useNavigate();
  const selectedUserRef = useRef(selectedUser);

  // Keep ref updated
  useEffect(() => {
    selectedUserRef.current = selectedUser;
  }, [selectedUser]);

  useEffect(() => {
    // Always load users first, then conversations for selected user
    loadUsers();
    
    // 设置定时刷新 - 每30秒刷新一次用户数据和会话
    const interval = setInterval(() => {
      loadUsers();
      if (selectedUserRef.current) {
        loadConversationsForUser(selectedUserRef.current.id);
      }
    }, 30000); // 30秒
    
    return () => clearInterval(interval);
  }, []); // 空依赖数组，只执行一次

  const loadUsers = useCallback(async () => {
    try {
      const response = await axios.get('/api/v1/users');
      setUsers(response.data.users || []);
      // Auto-select the first user if exists and no user is currently selected
      if (response.data.users && response.data.users.length > 0 && !selectedUserRef.current) {
        const firstUser = response.data.users[0];
        setSelectedUser(firstUser);
        loadConversationsForUser(firstUser.id);
      }
    } catch (error) {
      console.error('Failed to load users:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadConversationsForUser = useCallback(async (userID) => {
    try {
      const response = await axios.get(`/api/v1/users/${userID}/conversations`);
      setConversations(response.data.conversations || []);
      setSyncStatus(prev => ({
        ...prev,
        totalConversations: response.data.conversations?.length || 0
      }));
    } catch (error) {
      console.error('Failed to load conversations:', error);
    }
  }, []);

  const handleUserSelect = (user) => {
    setSelectedUser(user);
    setSelectedConversation(null); // Clear conversation selection
    loadConversationsForUser(user.id);
  };

  const handleSync = async () => {
    if (!isAuthenticated) {
      alert('请先登录 Telegram 才能同步数据');
      navigate('/login');
      return;
    }
    
    setSyncing(true);
    try {
      const response = await axios.post('/api/v1/sync');
      if (response.data.success) {
        // 重新加载会话列表
        setTimeout(() => {
          if (selectedUser) {
            loadConversationsForUser(selectedUser.id);
          }
        }, 2000);
      }
    } catch (error) {
      console.error('Sync failed:', error);
    } finally {
      setSyncing(false);
    }
  };

  const handleLogout = () => {
    setShowLogoutConfirm(true);
    setConfirmText('');
  };

  const confirmLogout = () => {
    if (confirmText === '确认删除') {
      logout();
      setShowLogoutConfirm(false);
      setConfirmText('');
      // Stay on main page after logout
    }
  };

  const cancelLogout = () => {
    setShowLogoutConfirm(false);
    setConfirmText('');
  };

  const getConversationIcon = (type) => {
    switch (type) {
      case 'user':
        return <FaUser />;
      case 'bot':
        return <FaRobot />;
      case 'group':
        return <FaUsers />;
      case 'channel':
        return <FaBroadcastTower />;
      default:
        return <FaUser />;
    }
  };

  if (loading) {
    return (
      <MainContainer>
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <div>加载中...</div>
        </div>
      </MainContainer>
    );
  }

  return (
    <MainContainer>
      <Sidebar>
        <SidebarHeader>
          <Title>
            <FaBroadcastTower color="#0088cc" />
            {selectedUser ? `${selectedUser.first_name} ${selectedUser.last_name}`.trim() : '选择用户'}
          </Title>
          <ButtonGroup>
            {selectedUser && (
              <ActionButton 
                className="sync" 
                onClick={handleSync} 
                disabled={syncing}
                title={!isAuthenticated ? '请先登录 Telegram' : ''}
              >
                <FaSync className={syncing ? 'spin' : ''} />
                {syncing ? '同步中...' : '同步'}
              </ActionButton>
            )}
            
            <ActionButton className="sync" onClick={() => navigate('/login')}>
              <FaUser />
              新增备份
            </ActionButton>
            
            {selectedUser && isAuthenticated && (
              <ActionButton className="logout" onClick={handleLogout}>
                <FaSignOutAlt />
                退出
              </ActionButton>
            )}
          </ButtonGroup>
        </SidebarHeader>
        
        {users.length > 0 && (
          <UserList>
            <UserListTitle>已备份用户</UserListTitle>
            {users.map(user => (
              <UserItem 
                key={user.id} 
                selected={selectedUser?.id === user.id}
                onClick={() => handleUserSelect(user)}
              >
                <UserName>
                  {`${user.first_name} ${user.last_name}`.trim() || user.username || `用户 ${user.id}`}
                </UserName>
                <UserStatus active={user.is_active}>
                  {user.is_active ? '可用' : '会话已失效'} • 
                  最后同步: {user.last_sync_time ? new Date(user.last_sync_time).toLocaleDateString() : '未同步'}
                </UserStatus>
              </UserItem>
            ))}
          </UserList>
        )}
        
        <ConversationList 
          conversations={conversations}
          selectedConversation={selectedConversation}
          onSelectConversation={setSelectedConversation}
          getConversationIcon={getConversationIcon}
        />
      </Sidebar>

      <ChatContainer>
        {selectedConversation ? (
          <ChatView conversation={selectedConversation} />
        ) : (
          <EmptyState>
            请选择一个会话查看聊天记录
          </EmptyState>
        )}
      </ChatContainer>

      <StatusBar>
        会话数: {syncStatus.totalConversations} | 消息数: {syncStatus.totalMessages}
        {syncStatus.lastSyncTime && ` | 最后同步: ${new Date(syncStatus.lastSyncTime).toLocaleString()}`}
      </StatusBar>

      {showLogoutConfirm && (
        <ConfirmModal>
          <ConfirmDialog>
            <ConfirmTitle>确认退出</ConfirmTitle>
            <ConfirmText>
              此操作将清除本地登录状态。如需继续，请在下方输入框中输入 <strong>"确认删除"</strong>
            </ConfirmText>
            <ConfirmInput
              type="text"
              placeholder="请输入：确认删除"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && confirmText === '确认删除') {
                  confirmLogout();
                }
              }}
            />
            <ConfirmButtons>
              <ConfirmButton className="cancel" onClick={cancelLogout}>
                取消
              </ConfirmButton>
              <ConfirmButton 
                className="confirm" 
                onClick={confirmLogout}
                disabled={confirmText !== '确认删除'}
              >
                确认退出
              </ConfirmButton>
            </ConfirmButtons>
          </ConfirmDialog>
        </ConfirmModal>
      )}
    </MainContainer>
  );
};

export default MainPage;