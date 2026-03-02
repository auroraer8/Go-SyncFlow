// 管理后台主题配置
export interface AdminTheme {
  value: string;
  label: string;
  preview: {
    sidebar: string;
    header: string;
    content: string;
  };
}

export const adminThemes: AdminTheme[] = [
  {
    value: 'default',
    label: '默认',
    preview: { sidebar: '#1f2937', header: '#3b82f6', content: '#f8fafc' }
  },
  {
    value: 'white',
    label: '纯白',
    preview: { sidebar: '#ffffff', header: '#3b82f6', content: '#f8fafc' }
  },
  {
    value: 'light-blue',
    label: '浅蓝',
    preview: { sidebar: '#f0f9ff', header: '#0ea5e9', content: '#f8fafc' }
  },
  {
    value: 'dark-blue',
    label: '深蓝',
    preview: { sidebar: '#1e3a5f', header: '#60a5fa', content: '#f1f5f9' }
  },
  {
    value: 'purple',
    label: '紫色',
    preview: { sidebar: '#4c1d95', header: '#8b5cf6', content: '#faf5ff' }
  },
  {
    value: 'green',
    label: '绿色',
    preview: { sidebar: '#14532d', header: '#22c55e', content: '#f0fdf4' }
  },
  {
    value: 'gray',
    label: '灰色',
    preview: { sidebar: '#374151', header: '#6b7280', content: '#f3f4f6' }
  },
  {
    value: 'dark',
    label: '深色',
    preview: { sidebar: '#111827', header: '#3b82f6', content: '#1f2937' }
  }
];

// 登录页主题配置
export interface LoginTheme {
  value: string;
  label: string;
  preview: string;
}

export const loginThemes: LoginTheme[] = [
  {
    value: 'light',
    label: '浅色',
    preview: 'linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%)'
  },
  {
    value: 'dark',
    label: '深色',
    preview: 'linear-gradient(135deg, #0f172a 0%, #1e3a8a 100%)'
  },
  {
    value: 'blue',
    label: '蓝色',
    preview: 'linear-gradient(135deg, #1e40af 0%, #3b82f6 100%)'
  }
];

// 应用管理后台主题
export function applyAdminTheme(theme: string) {
  const root = document.documentElement;
  root.setAttribute('data-theme', theme || 'default');
  localStorage.setItem('admin-theme', theme || 'default');
}

// 获取当前管理后台主题
export function getCurrentAdminTheme(): string {
  return localStorage.getItem('admin-theme') || 'default';
}

// 初始化主题（在应用启动时调用）
export function initTheme() {
  const savedTheme = localStorage.getItem('admin-theme');
  if (savedTheme) {
    applyAdminTheme(savedTheme);
  }
}
