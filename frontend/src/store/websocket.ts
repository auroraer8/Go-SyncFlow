import { defineStore } from 'pinia';
import { ref, computed } from 'vue';

// 事件类型定义
export type WSEventType = 
  | 'system_metrics'      // 系统指标更新
  | 'business_stats'      // 业务统计更新
  | 'user_change'         // 用户变更（创建/更新/删除/状态变更）
  | 'group_change'        // 群组变更
  | 'role_change'         // 角色变更
  | 'sync_event'          // 同步事件（开始/进度/完成）
  | 'sync_log'            // 新同步日志
  | 'connector_change'    // 连接器变更
  | 'sso_change'          // SSO应用变更
  | 'notification';       // 系统通知

export interface WSMessage {
  type: WSEventType;
  timestamp: number;
  data: any;
}

export type EventCallback = (data: any) => void;

export const useWebSocketStore = defineStore('websocket', () => {
  // 状态
  const connected = ref(false);
  const usePolling = ref(false);
  const reconnectCount = ref(0);
  
  // WebSocket 实例
  let ws: WebSocket | null = null;
  let reconnectTimer: number | null = null;
  let pollingTimer: number | null = null;
  let pingTimer: number | null = null;
  
  // 事件订阅者
  const subscribers = new Map<WSEventType, Set<EventCallback>>();
  
  // 连接状态
  const status = computed(() => {
    if (connected.value && !usePolling.value) return 'realtime';
    if (connected.value && usePolling.value) return 'polling';
    return 'offline';
  });
  
  // 订阅事件
  const subscribe = (eventType: WSEventType, callback: EventCallback) => {
    if (!subscribers.has(eventType)) {
      subscribers.set(eventType, new Set());
    }
    subscribers.get(eventType)!.add(callback);
    
    // 返回取消订阅函数
    return () => {
      subscribers.get(eventType)?.delete(callback);
    };
  };
  
  // 批量订阅
  const subscribeMany = (events: WSEventType[], callback: EventCallback) => {
    const unsubscribes = events.map(e => subscribe(e, callback));
    return () => unsubscribes.forEach(fn => fn());
  };
  
  // 触发事件
  const emit = (eventType: WSEventType, data: any) => {
    const callbacks = subscribers.get(eventType);
    if (callbacks) {
      callbacks.forEach(cb => {
        try {
          cb(data);
        } catch (e) {
          console.error(`[WebSocket] 事件处理错误 (${eventType}):`, e);
        }
      });
    }
  };
  
  // 启动 HTTP 轮询（降级方案）
  const startPolling = async () => {
    if (pollingTimer) return;
    
    console.log('[WebSocket] 启用 HTTP 轮询模式');
    usePolling.value = true;
    connected.value = true;
    
    const poll = async () => {
      try {
        const response = await fetch('/api/system/status');
        const result = await response.json();
        if (result.success) {
          emit('system_metrics', {
            cpu: result.data.cpu,
            memory: result.data.memory,
            disk: result.data.disk,
            network: result.data.network
          });
        }
        
        const statsResponse = await fetch('/api/dashboard/stats');
        const statsResult = await statsResponse.json();
        if (statsResult.success) {
          emit('business_stats', statsResult.data);
        }
      } catch (e) {
        console.warn('[WebSocket] 轮询失败:', e);
      }
    };
    
    poll();
    pollingTimer = window.setInterval(poll, 5000);
  };
  
  const stopPolling = () => {
    if (pollingTimer) {
      clearInterval(pollingTimer);
      pollingTimer = null;
    }
    usePolling.value = false;
  };
  
  // 连接 WebSocket
  const connect = () => {
    if (ws && ws.readyState === WebSocket.OPEN) return;
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws/dashboard`;
    
    try {
      ws = new WebSocket(wsUrl);
    } catch (e) {
      console.warn('[WebSocket] 创建失败，切换到轮询模式');
      startPolling();
      return;
    }
    
    ws.onopen = () => {
      console.log('[WebSocket] 已连接');
      connected.value = true;
      reconnectCount.value = 0;
      stopPolling();
      
      // 启动心跳
      startPing();
    };
    
    ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        emit(msg.type, msg.data);
      } catch (e) {
        console.warn('[WebSocket] 消息解析失败:', e);
      }
    };
    
    ws.onclose = () => {
      console.log('[WebSocket] 已断开');
      connected.value = false;
      stopPing();
      
      // 自动重连（最多 3 次，然后降级到轮询）
      if (reconnectCount.value < 3) {
        const delay = Math.min(1000 * Math.pow(2, reconnectCount.value), 5000);
        reconnectTimer = window.setTimeout(() => {
          reconnectCount.value++;
          connect();
        }, delay);
      } else {
        console.log('[WebSocket] 重连失败，切换到轮询模式');
        startPolling();
      }
    };
    
    ws.onerror = (error) => {
      console.error('[WebSocket] 错误:', error);
    };
  };
  
  // 心跳保活
  const startPing = () => {
    stopPing();
    pingTimer = window.setInterval(() => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'ping' }));
      }
    }, 30000);
  };
  
  const stopPing = () => {
    if (pingTimer) {
      clearInterval(pingTimer);
      pingTimer = null;
    }
  };
  
  // 断开连接
  const disconnect = () => {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    stopPolling();
    stopPing();
    if (ws) {
      ws.close();
      ws = null;
    }
    connected.value = false;
  };
  
  // 发送消息
  const send = (message: any) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message));
    }
  };
  
  return {
    // 状态
    connected,
    usePolling,
    status,
    
    // 方法
    connect,
    disconnect,
    subscribe,
    subscribeMany,
    send
  };
});
