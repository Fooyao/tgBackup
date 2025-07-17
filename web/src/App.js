import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import styled from 'styled-components';
import LoginPage from './pages/LoginPage';
import MainPage from './pages/MainPage';
import { AuthProvider } from './hooks/useAuth';

const AppContainer = styled.div`
  height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  display: flex;
  flex-direction: column;
`;

function App() {
  return (
    <AuthProvider>
      <AppContainer>
        <Router>
          <Routes>
            <Route path="/" element={<MainPage />} />
            <Route path="/login" element={<LoginPage />} />
          </Routes>
        </Router>
      </AppContainer>
    </AuthProvider>
  );
}

export default App;