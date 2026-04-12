import './App.css'
import {h} from 'preact';
import {useState, useEffect, useRef} from "preact/compat";
import {Switch, Version, ProxyList, LoadSubscription, LoadLocalFile, GetAccountStatus} from "../wailsjs/go/main/App";

declare global {
    interface Window {
        Switch: (enable: boolean, proxy: string, router: boolean) => Promise<string>;
        Version: () => Promise<string>;
        ProxyList: () => Promise<string[]>;
        LoadSubscription: (url: string) => Promise<ProxyNode[]>;
        LoadLocalFile: (content: string) => Promise<ProxyNode[]>;
        GetAccountStatus: () => Promise<string>;
    }
}

interface TrafficData {
    up: number;
    down: number;
}

interface ProxyNode {
    name: string;
    protocol: string;
    method?: string;
    password?: string;
    host: string;
    port: number;
}

type SourceType = 'api' | 'local' | 'subscription';

export function App(props: any) {
    const [status, setStatus] = useState('开始加速');
    const [isLoading, setIsLoading] = useState(false);
    const [getVersion, setVersion] = useState("v0.0.1");
    const [selectedProxy, setSelectedProxy] = useState('');
    const [proxyList, setProxyList] = useState<ProxyNode[]>([]);
    const [proxyNames, setProxyNames] = useState<string[]>([]);
    const [isAccelerated, setIsAccelerated] = useState(false);
    const [isHostMode, setIsHostMode] = useState(false);
    const [sourceType, setSourceType] = useState<SourceType>('api');
    const [subscriptionUrl, setSubscriptionUrl] = useState('');
    const [isLoadingProxies, setIsLoadingProxies] = useState(false);
    const [stats, setStats] = useState({download: 0, upload: 0, totalTraffic: 0, uptime: 0});
    const [showAccountModal, setShowAccountModal] = useState(false);
    const [showSettingsModal, setShowSettingsModal] = useState(false);
    const [logs, setLogs] = useState<string>('');
    const [isLoadingLogs, setIsLoadingLogs] = useState(false);
    const timerRef = useRef<number | null>(null);
    const wsRef = useRef<WebSocket | null>(null);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
    };

    const formatSpeed = (bytesPerSecond: number) => {
        if (bytesPerSecond === 0) return '0 B/s';
        const units = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
        const i = Math.floor(Math.log(bytesPerSecond) / Math.log(1024));
        return (bytesPerSecond / Math.pow(1024, i)).toFixed(1) + ' ' + units[Math.min(i, units.length - 1)];
    };

    const loadProxies = async (type: SourceType, data?: string) => {
        setIsLoadingProxies(true);
        try {
            if (type === 'api') {
                const servers = await ProxyList();
                const nodes = servers.map(name => ({ name, protocol: '', host: '', port: 0, password: '' }));
                setProxyList(nodes);
                setProxyNames(servers);
                if (servers.length > 0) setSelectedProxy(servers[0]);
            } else if (type === 'subscription' && data) {
                const nodes = await LoadSubscription(data);
                if (nodes && nodes.length > 0) {
                    setProxyList(nodes);
                    setProxyNames(nodes.map(n => n.name));
                    if (nodes.length > 0) setSelectedProxy(nodes[0].name);
                }
            } else if (type === 'local' && data) {
                const nodes = await LoadLocalFile(data);
                if (nodes && nodes.length > 0) {
                    setProxyList(nodes);
                    setProxyNames(nodes.map(n => n.name));
                    if (nodes.length > 0) setSelectedProxy(nodes[0].name);
                }
            }
        } catch (error) {
            console.error('加载节点失败:', error);
        } finally {
            setIsLoadingProxies(false);
        }
    };

    const handleFileUpload = async (e: Event) => {
        const input = e.target as HTMLInputElement;
        const file = input.files?.[0];
        if (!file) return;

        try {
            const text = await file.text();
            const nodes = await LoadLocalFile(text);
            if (nodes && nodes.length > 0) {
                setProxyList(nodes);
                setProxyNames(nodes.map(n => n.name));
                if (nodes.length > 0) setSelectedProxy(nodes[0].name);
            }
        } catch (error) {
            console.error('解析节点文件失败:', error);
        }
    };

    const handleSubscriptionLoad = () => {
        if (subscriptionUrl.trim()) {
            loadProxies('subscription', subscriptionUrl);
        }
    };

    const connectWebSocket = () => {
        disconnectWebSocket();
        const ws = new WebSocket('ws://127.0.0.1:54713/traffic');
        ws.onmessage = (event) => {
            try {
                const data: TrafficData = JSON.parse(event.data);
                const trafficSum = data.up + data.down;
                setStats(prev => ({
                    ...prev,
                    download: data.down,
                    upload: data.up,
                    totalTraffic: prev.totalTraffic + trafficSum
                }));
            } catch (error) {
                console.error('解析WebSocket数据失败:', error);
            }
        };
        wsRef.current = ws;
    };

    const disconnectWebSocket = () => {
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
            wsRef.current.close();
            wsRef.current = null;
        }
    };

    const updateStatus = () => {
        const newStatus = status === "开始加速" ? "停止加速" : "开始加速";
        setStatus(newStatus);
        setIsAccelerated(newStatus === "停止加速");
        if (newStatus === "停止加速") {
            if (timerRef.current !== null) {
                clearInterval(timerRef.current);
                timerRef.current = null;
            }
            connectWebSocket();
            setStats({download: 0, upload: 0, totalTraffic: 0, uptime: 0});
            timerRef.current = window.setInterval(() => {
                setStats(prev => ({...prev, uptime: prev.uptime + 1/60}));
            }, 1000);
        } else {
            disconnectWebSocket();
            if (timerRef.current !== null) {
                clearInterval(timerRef.current);
                timerRef.current = null;
            }
            setStats({download: 0, upload: 0, totalTraffic: 0, uptime: 0});
        }
    };

    const handleSwitch = () => {
        if (proxyList.length === 0) {
            alert('请先加载节点');
            return;
        }
        setIsLoading(true);
        Switch(status === "开始加速", selectedProxy, isHostMode).then((res) => {
            setIsLoading(false);
            if (res === "") {
                updateStatus();
            }
        });
    };

    useEffect(() => {
        return () => {
            if (timerRef.current !== null) clearInterval(timerRef.current);
            disconnectWebSocket();
        };
    }, []);

    useEffect(() => {
        async function init() {
            try {
                const version = await Version();
                setVersion(version);
                await loadProxies('api');
            } catch (error) {
                console.error("初始化失败:", error);
            }
        }
        init();
    }, []);

    useEffect(() => {
        if (proxyNames.length > 0 && !selectedProxy) {
            setSelectedProxy(proxyNames[0]);
        }
    }, [proxyNames]);

    const getUptimeDisplay = () => {
        const minutes = Math.floor(stats.uptime);
        const seconds = Math.floor((stats.uptime % 1) * 60);
        return `${minutes}分${seconds}秒`;
    };

    const loadLogs = async () => {
        setIsLoadingLogs(true);
        try {
            // 模拟加载日志，实际应该调用后端API
            // 这里使用setTimeout模拟网络请求
            setTimeout(() => {
                const mockLogs = `[2026-04-13 10:00:00] 启动YuLiReBa加速器
[2026-04-13 10:00:01] 加载配置文件成功
[2026-04-13 10:00:02] 连接到API服务器
[2026-04-13 10:00:03] 加载节点列表成功
[2026-04-13 10:00:04] 初始化完成
[2026-04-13 10:05:00] 选择节点: 节点1
[2026-04-13 10:05:01] 开始加速
[2026-04-13 10:05:02] 连接成功
[2026-04-13 10:10:00] 停止加速
[2026-04-13 10:10:01] 断开连接`;
                setLogs(mockLogs);
                setIsLoadingLogs(false);
            }, 1000);
        } catch (error) {
            console.error('加载日志失败:', error);
            setIsLoadingLogs(false);
        }
    };

    return (
        <div className="app-container">
            <div className="particles-bg" id="particles-background"></div>
            
            <header className="top-header">
                <div className="logo">
                    <span className="logo-highlight">YuLi</span>ReBa
                </div>
                <div className="header-buttons">
                    <button className="settings-btn" onClick={() => setShowSettingsModal(true)}>
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="3"/>
                            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
                        </svg>
                        <span>设置</span>
                    </button>
                    <button className="account-btn" onClick={() => setShowAccountModal(true)}>
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                            <circle cx="12" cy="7" r="4"/>
                        </svg>
                        <span>账户</span>
                    </button>
                </div>
            </header>

            <main className="main-content">
                <div className="bento-grid">
                    <div className="bento-card proxy-select-card">
                        <div className="card-header">
                            <h3>加速节点</h3>
                        </div>
                        <div className="card-body">
                            <select 
                                className="proxy-select"
                                value={selectedProxy}
                                onChange={(e) => setSelectedProxy((e.target as HTMLSelectElement).value)}
                                disabled={isAccelerated || proxyList.length === 0}
                            >
                                {proxyNames.length === 0 ? (
                                    <option value="">暂无节点</option>
                                ) : (
                                    proxyNames.map((name, idx) => (
                                        <option key={idx} value={name}>{name}</option>
                                    ))
                                )}
                            </select>
                            <button 
                                className="refresh-btn"
                                onClick={() => loadProxies(sourceType, subscriptionUrl)}
                                disabled={isLoadingProxies}
                            >
                                <svg className={isLoadingProxies ? 'spinning' : ''} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                    <path d="M23 4v6h-6M1 20v-6h6"/>
                                    <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                                </svg>
                            </button>
                        </div>
                    </div>

                    <div className="bento-card control-card">
                        <div className="card-header">
                            <h3>加速控制</h3>
                            <div className={`status-badge ${isAccelerated ? 'active' : ''}`}>
                                {isAccelerated ? '加速中' : '未加速'}
                            </div>
                        </div>
                        <div className="card-body center">
                            <button 
                                className={`switch-btn ${isAccelerated ? 'active' : ''} ${isLoading ? 'loading' : ''}`}
                                onClick={handleSwitch}
                                disabled={isLoading || proxyList.length === 0}
                            >
                                {!isLoading && (
                                    <svg width="32" height="32" viewBox="0 0 24 24" fill="currentColor">
                                        {isAccelerated ? (
                                            <rect x="6" y="6" width="12" height="12" rx="2"/>
                                        ) : (
                                            <polygon points="5,3 19,12 5,21"/>
                                        )}
                                    </svg>
                                )}
                                {isLoading ? '加载中...' : status}
                            </button>
                            {!isAccelerated && (
                                <label className="host-mode-toggle">
                                    <input
                                        type="checkbox"
                                        checked={isHostMode}
                                        onChange={(e) => setIsHostMode((e.target as HTMLInputElement).checked)}
                                    />
                                    <span>主机模式</span>
                                </label>
                            )}
                        </div>
                    </div>

                    <div className="bento-card source-card">
                        <div className="card-header">
                            <h3>节点来源</h3>
                        </div>
                        <div className="card-body">
                            <div className="source-tabs">
                                <button 
                                    className={`tab ${sourceType === 'api' ? 'active' : ''}`}
                                    onClick={() => setSourceType('api')}
                                >
                                    API
                                </button>
                                <button 
                                    className={`tab ${sourceType === 'local' ? 'active' : ''}`}
                                    onClick={() => setSourceType('local')}
                                >
                                    本地文件
                                </button>
                                <button 
                                    className={`tab ${sourceType === 'subscription' ? 'active' : ''}`}
                                    onClick={() => setSourceType('subscription')}
                                >
                                    订阅链接
                                </button>
                            </div>
                            
                            <div className="source-content">
                                {sourceType === 'api' && (
                                    <p className="source-hint">从服务器 API 加载节点配置</p>
                                )}
                                {sourceType === 'local' && (
                                    <div className="file-upload-area">
                                        <input
                                            type="file"
                                            ref={fileInputRef}
                                            accept=".json,.yml,.yaml"
                                            onChange={handleFileUpload}
                                            style={{display: 'none'}}
                                        />
                                        <button className="upload-btn" onClick={() => fileInputRef.current?.click()}>
                                            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                                                <polyline points="17 8 12 3 7 8"/>
                                                <line x1="12" y1="3" x2="12" y2="15"/>
                                            </svg>
                                            选择文件
                                        </button>
                                        <p className="file-hint">支持 JSON 格式的节点文件</p>
                                    </div>
                                )}
                                {sourceType === 'subscription' && (
                                    <div className="subscription-input">
                                        <input
                                            type="text"
                                            placeholder="输入订阅链接..."
                                            value={subscriptionUrl}
                                            onChange={(e) => setSubscriptionUrl((e.target as HTMLInputElement).value)}
                                        />
                                        <button className="load-btn" onClick={handleSubscriptionLoad}>
                                            加载
                                        </button>
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>

                    <div className="bento-card stats-card">
                        <div className="card-header">
                            <h3>流量统计</h3>
                        </div>
                        <div className="card-body">
                            <div className="stats-grid">
                                <div className="stat-item">
                                    <span className="stat-label">下载</span>
                                    <span className="stat-value download">{formatSpeed(stats.download)}</span>
                                </div>
                                <div className="stat-item">
                                    <span className="stat-label">上传</span>
                                    <span className="stat-value upload">{formatSpeed(stats.upload)}</span>
                                </div>
                                <div className="stat-item">
                                    <span className="stat-label">总流量</span>
                                    <span className="stat-value">{formatBytes(stats.totalTraffic)}</span>
                                </div>
                                <div className="stat-item">
                                    <span className="stat-label">运行时长</span>
                                    <span className="stat-value">{getUptimeDisplay()}</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </main>

            <footer className="app-footer">
                <span>YuLiReBa 加速器</span>
                <span className="version">{getVersion}</span>
            </footer>

            {showAccountModal && (
                <div className="modal-overlay" onClick={() => setShowAccountModal(false)}>
                    <div className="modal-content" onClick={(e) => e.stopPropagation()}>
                        <div className="modal-header">
                            <h2>账户系统</h2>
                            <button className="close-btn" onClick={() => setShowAccountModal(false)}>
                                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                    <line x1="18" y1="6" x2="6" y2="18"/>
                                    <line x1="6" y1="6" x2="18" y2="18"/>
                                </svg>
                            </button>
                        </div>
                        <div className="modal-body">
                            <div className="account-placeholder">
                                <div className="placeholder-icon">
                                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                                        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                                        <circle cx="12" cy="7" r="4"/>
                                    </svg>
                                </div>
                                <h3>账户系统开发中</h3>
                                <p>基于 Supabase 的账户系统即将上线</p>
                                <div className="feature-list">
                                    <div className="feature-item">
                                        <span className="check">✓</span>
                                        <span>用户注册/登录</span>
                                    </div>
                                    <div className="feature-item">
                                        <span className="check">✓</span>
                                        <span>节点订阅同步</span>
                                    </div>
                                    <div className="feature-item">
                                        <span className="check">✓</span>
                                        <span>使用时长管理</span>
                                    </div>
                                    <div className="feature-item">
                                        <span className="pending">⏳</span>
                                        <span>即将上线...</span>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {showSettingsModal && (
                <div className="modal-overlay" onClick={() => setShowSettingsModal(false)}>
                    <div className="modal-content settings-modal" onClick={(e) => e.stopPropagation()}>
                        <div className="modal-header">
                            <h2>设置</h2>
                            <button className="close-btn" onClick={() => setShowSettingsModal(false)}>
                                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                    <line x1="18" y1="6" x2="6" y2="18"/>
                                    <line x1="6" y1="6" x2="18" y2="18"/>
                                </svg>
                            </button>
                        </div>
                        <div className="modal-body">
                            <div className="settings-section">
                                <h3>工作日志</h3>
                                <div className="logs-container">
                                    {isLoadingLogs ? (
                                        <div className="loading-logs">加载中...</div>
                                    ) : (
                                        <pre className="logs-content">{logs || '暂无日志'}</pre>
                                    )}
                                </div>
                                <button className="load-logs-btn" onClick={loadLogs} disabled={isLoadingLogs}>
                                    {isLoadingLogs ? '加载中...' : '刷新日志'}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
