import { useState, useEffect } from 'react';
import { Activity, Play, List, User, Shield, Terminal } from 'lucide-react';
import { useSSE } from './hooks/useSSE';
import DAGViewer from './components/DAGViewer';
import './App.css';

function App() {
  const [activeTab, setActiveTab] = useState('workflows');
  const [workflows, setWorkflows] = useState<any[]>([]);
  const sseData = useSSE('http://localhost:8080/api/v1/monitor/stream');

  useEffect(() => {
    // Mocking workflows for now
    setWorkflows([
      { id: 'wf-1', name: 'User Onboarding', status: 'SUCCESS' },
      { id: 'wf-2', name: 'Payment Process', status: 'RUNNING' },
      { id: 'wf-3', name: 'Cleanup Task', status: 'IDLE' },
    ]);
  }, []);

  const steps = [
    { id: 'step-1', action: 'HTTP', status: sseData?.step_id === 'step-1' ? sseData.status : 'SUCCESS' },
    { id: 'step-2', action: 'WAIT', status: sseData?.step_id === 'step-2' ? sseData.status : 'RUNNING', depends_on: ['step-1'] },
    { id: 'step-3', action: 'TRANSFORM', status: sseData?.step_id === 'step-3' ? sseData.status : 'IDLE', depends_on: ['step-2'] },
  ];

  return (
    <div className="app-container">
      <nav className="nav-container">
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <div style={{ padding: '8px', background: 'var(--primary)', borderRadius: '12px' }}>
            <Activity size={24} color="white" />
          </div>
          <h1 style={{ fontSize: '24px', fontWeight: 700, letterSpacing: '-0.02em' }}>FlowForge</h1>
        </div>
        <div style={{ display: 'flex', gap: '24px' }}>
          <button className="glow-btn" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Play size={18} /> Run Workflow
          </button>
          <div style={{ width: '40px', height: '40px', borderRadius: '50%', background: 'var(--border)' }} />
        </div>
      </nav>

      <div className="dashboard-layout">
        <aside className="sidebar">
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <NavItem icon={<List size={20} />} label="Workflows" active={activeTab === 'workflows'} onClick={() => setActiveTab('workflows')} />
            <NavItem icon={<Activity size={20} />} label="Executions" active={activeTab === 'executions'} onClick={() => setActiveTab('executions')} />
            <NavItem icon={<Shield size={20} />} label="Security" active={activeTab === 'security'} onClick={() => setActiveTab('security')} />
            <NavItem icon={<User size={20} />} label="Users" active={activeTab === 'users'} onClick={() => setActiveTab('users')} />
          </div>
        </aside>

        <main className="main-content">
          <div style={{ marginBottom: '40px' }}>
            <h2 style={{ fontSize: '32px', marginBottom: '8px' }}>Real-time Monitoring</h2>
            <p style={{ color: 'var(--text-secondary)' }}>Observe and collaborate on automated workflows as they happen.</p>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 340px', gap: '32px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '32px' }}>
              <DAGViewer steps={steps} />
              
              <div className="glass-card" style={{ padding: '24px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '20px' }}>
                  <Terminal size={20} color="var(--primary)" />
                  <h3 style={{ fontSize: '18px' }}>Execution Log</h3>
                </div>
                <div style={{ background: '#000', borderRadius: '8px', padding: '16px', fontFamily: 'monospace', fontSize: '14px', height: '200px', overflowY: 'auto', border: '1px solid var(--border)' }}>
                  <div style={{ color: '#aaa' }}>[12:00:01] Starting execution exec-parallel...</div>
                  <div style={{ color: '#aaa' }}>[12:00:01] Running step-1 (HTTP)...</div>
                  <div style={{ color: '#10b981' }}>[12:00:02] step-1 completed successfully.</div>
                  <div style={{ color: '#aaa' }}>[12:00:02] Running step-2 (WAIT)...</div>
                  {sseData && (
                    <div style={{ color: sseData.status === 'FAILED' ? '#f43f5e' : '#6366f1' }}>
                      [{new Date().toLocaleTimeString()}] {sseData.step_id} status updated to {sseData.status}
                    </div>
                  )}
                </div>
              </div>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
              <div className="glass-card" style={{ padding: '24px' }}>
                <h3 style={{ fontSize: '18px', marginBottom: '16px' }}>Active Workflows</h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                  {workflows.map(wf => (
                    <div key={wf.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span style={{ fontSize: '14px' }}>{wf.name}</span>
                      <span className={`status-badge status-${wf.status.toLowerCase()}`}>{wf.status}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="glass-card" style={{ padding: '24px', background: 'linear-gradient(135deg, var(--primary-glow), transparent)' }}>
                <h3 style={{ fontSize: '18px', marginBottom: '12px' }}>Quick Stats</h3>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                  <StatCard label="Success Rate" value="98.2%" />
                  <StatCard label="Avg Time" value="4.2s" />
                </div>
              </div>
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}

const NavItem = ({ icon, label, active, onClick }: any) => (
  <div 
    onClick={onClick}
    style={{ 
      display: 'flex', 
      alignItems: 'center', 
      gap: '12px', 
      padding: '12px 16px', 
      borderRadius: '8px',
      cursor: 'pointer',
      background: active ? 'rgba(99, 102, 241, 0.1)' : 'transparent',
      color: active ? 'var(--primary)' : 'var(--text-secondary)',
      transition: 'all 0.2s ease',
    }}
    className="nav-item"
  >
    {icon}
    <span style={{ fontWeight: 600 }}>{label}</span>
  </div>
);

const StatCard = ({ label, value }: any) => (
  <div style={{ padding: '12px', borderRadius: '12px', background: 'rgba(255,255,255,0.02)' }}>
    <div style={{ fontSize: '12px', color: 'var(--text-secondary)', marginBottom: '4px' }}>{label}</div>
    <div style={{ fontSize: '20px', fontWeight: 700 }}>{value}</div>
  </div>
);

export default App;
