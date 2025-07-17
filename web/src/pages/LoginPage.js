import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import styled from 'styled-components';
import QRCode from 'react-qr-code';
import { FaTelegram, FaPhone, FaQrcode } from 'react-icons/fa';
import { useAuth } from '../hooks/useAuth';

const LoginContainer = styled.div`
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  padding: 20px;
`;

const LoginCard = styled.div`
  background: white;
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
  width: 100%;
  max-width: 400px;
`;

const Title = styled.h1`
  text-align: center;
  color: #333;
  margin-bottom: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
`;

const TabContainer = styled.div`
  display: flex;
  margin-bottom: 30px;
  border-radius: 8px;
  overflow: hidden;
  background: #f5f5f5;
`;

const Tab = styled.button`
  flex: 1;
  padding: 12px;
  border: none;
  background: ${props => props.active ? '#0088cc' : 'transparent'};
  color: ${props => props.active ? 'white' : '#666'};
  cursor: pointer;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  
  &:hover {
    background: ${props => props.active ? '#0088cc' : '#e0e0e0'};
  }
`;

const FormGroup = styled.div`
  margin-bottom: 20px;
`;

const Label = styled.label`
  display: block;
  margin-bottom: 5px;
  color: #333;
  font-weight: 500;
`;

const Input = styled.input`
  width: 100%;
  padding: 12px;
  border: 2px solid #e0e0e0;
  border-radius: 8px;
  font-size: 16px;
  transition: border-color 0.3s ease;
  
  &:focus {
    outline: none;
    border-color: #0088cc;
  }
`;

const Button = styled.button`
  width: 100%;
  padding: 14px;
  background: #0088cc;
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.3s ease;
  
  &:hover {
    background: #0077b5;
  }
  
  &:disabled {
    background: #ccc;
    cursor: not-allowed;
  }
`;

const QRContainer = styled.div`
  display: flex;
  justify-content: center;
  margin-bottom: 20px;
`;

const Message = styled.div`
  padding: 12px;
  border-radius: 8px;
  margin-bottom: 20px;
  text-align: center;
  
  &.success {
    background: #d4edda;
    color: #155724;
    border: 1px solid #c3e6cb;
  }
  
  &.error {
    background: #f8d7da;
    color: #721c24;
    border: 1px solid #f5c6cb;
  }
`;


const LoginPage = () => {
  const [activeTab, setActiveTab] = useState('phone');
  const [formData, setFormData] = useState({
    phone: '',
    code: ''
  });
  const [qrCode, setQrCode] = useState('');
  const [phoneHash, setPhoneHash] = useState('');
  const [showCodeInput, setShowCodeInput] = useState(false);
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  
  const { login, verifyCode, isAuthenticated } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/');
    }
  }, [isAuthenticated, navigate]);

  const handleInputChange = (e) => {
    setFormData({
      ...formData,
      [e.target.name]: e.target.value
    });
  };

  const handleLogin = async (e) => {
    e.preventDefault();
    setLoading(true);
    setMessage('');

    try {
      const loginData = {
        phone: formData.phone,
        use_qr: activeTab === 'qr'
      };

      const response = await login(loginData);

      if (response.success) {
        if (activeTab === 'qr') {
          setQrCode(response.qr_code);
          setMessage('请使用Telegram扫描二维码登录');
          // Start polling for QR status
          startQRPolling();
        } else {
          setPhoneHash(response.phone_hash);
          setShowCodeInput(true);
          setMessage('验证码已发送到您的手机');
        }
      }
    } catch (error) {
      setMessage(error.error || '登录失败');
    } finally {
      setLoading(false);
    }
  };

  const handleVerifyCode = async (e) => {
    e.preventDefault();
    setLoading(true);

    try {
      const verificationData = {
        phone: formData.phone,
        code: formData.code
      };

      const response = await verifyCode(verificationData);

      if (response.success) {
        setMessage('登录成功！');
        navigate('/');
      }
    } catch (error) {
      setMessage(error.error || '验证码错误');
    } finally {
      setLoading(false);
    }
  };

  const startQRPolling = () => {
    const pollInterval = setInterval(async () => {
      try {
        const response = await fetch('/api/v1/auth/qr-status');
        const data = await response.json();
        
        if (data.success && data.authenticated) {
          clearInterval(pollInterval);
          setMessage('二维码登录成功！');
          setTimeout(() => {
            navigate('/');
          }, 1000);
        }
      } catch (error) {
        console.error('QR polling error:', error);
      }
    }, 2000); // Poll every 2 seconds

    // Stop polling after 5 minutes
    setTimeout(() => {
      clearInterval(pollInterval);
      if (qrCode) {
        setMessage('二维码已过期，请重新生成');
      }
    }, 300000);
  };

  return (
    <LoginContainer>
      <LoginCard>
        <Title>
          <FaTelegram color="#0088cc" />
          Telegram 备份工具
        </Title>

        <TabContainer>
          <Tab
            active={activeTab === 'phone'}
            onClick={() => setActiveTab('phone')}
          >
            <FaPhone />
            手机登录
          </Tab>
          <Tab
            active={activeTab === 'qr'}
            onClick={() => setActiveTab('qr')}
          >
            <FaQrcode />
            二维码登录
          </Tab>
        </TabContainer>

        {message && (
          <Message className={message.includes('成功') ? 'success' : 'error'}>
            {message}
          </Message>
        )}

        <form onSubmit={showCodeInput ? handleVerifyCode : handleLogin}>

          {activeTab === 'phone' && (
            <FormGroup>
              <Label>手机号码</Label>
              <Input
                type="tel"
                name="phone"
                value={formData.phone}
                onChange={handleInputChange}
                placeholder="+86 138 0000 0000"
                required
              />
            </FormGroup>
          )}

          {showCodeInput && (
            <FormGroup>
              <Label>验证码</Label>
              <Input
                type="text"
                name="code"
                value={formData.code}
                onChange={handleInputChange}
                placeholder="请输入验证码"
                required
              />
            </FormGroup>
          )}

          {activeTab === 'qr' && qrCode && (
            <QRContainer>
              <QRCode value={qrCode} size={200} />
            </QRContainer>
          )}

          <Button type="submit" disabled={loading}>
            {loading ? '处理中...' : showCodeInput ? '验证登录' : '开始登录'}
          </Button>
        </form>
      </LoginCard>
    </LoginContainer>
  );
};

export default LoginPage;