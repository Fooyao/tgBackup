import React, { createContext, useContext, useState, useEffect } from 'react';
import axios from 'axios';

const AuthContext = createContext();

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

export const AuthProvider = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    checkAuthStatus();
  }, []);

  const checkAuthStatus = async () => {
    try {
      const response = await axios.get('/api/v1/auth/status');
      setIsAuthenticated(response.data.authenticated);
    } catch (error) {
      console.error('Auth check failed:', error);
      setIsAuthenticated(false);
    } finally {
      setLoading(false);
    }
  };

  const login = async (loginData) => {
    try {
      const response = await axios.post('/api/v1/auth/login', loginData);
      return response.data;
    } catch (error) {
      throw error.response?.data || error.message;
    }
  };

  const verifyCode = async (verificationData) => {
    try {
      const response = await axios.post('/api/v1/auth/verify', verificationData);
      if (response.data.success) {
        setIsAuthenticated(true);
      }
      return response.data;
    } catch (error) {
      throw error.response?.data || error.message;
    }
  };

  const logout = () => {
    setIsAuthenticated(false);
  };

  const value = {
    isAuthenticated,
    loading,
    login,
    verifyCode,
    logout,
    checkAuthStatus
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
};