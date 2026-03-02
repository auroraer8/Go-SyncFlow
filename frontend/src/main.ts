import { createApp } from "vue";
import { createPinia } from "pinia";
import ElementPlus from "element-plus";
import zhCn from "element-plus/dist/locale/zh-cn.mjs";
import "element-plus/dist/index.css";
import "./styles/variables.css";
import "./styles/themes.css";
import "./styles/global.css";
import "./styles/corporate.css";
import "./styles/sso.css";

import App from "./App.vue";
import router from "./router";
import { api } from "./api";
import { applyAdminTheme } from "./utils/theme";

const app = createApp(App);

app.use(createPinia());
app.use(ElementPlus, { locale: zhCn });
app.use(router);

// 动态设置浏览器标题和主题（异步，不阻塞渲染）
api.get("/settings/ui").then((res) => {
  if (res.data.success && res.data.data) {
    const config = res.data.data;
    if (config.browserTitle) {
      document.title = config.browserTitle;
    }
    if (config.adminTheme) {
      applyAdminTheme(config.adminTheme);
    }
  }
}).catch(() => {
  // 从本地存储恢复主题
  const savedTheme = localStorage.getItem('admin-theme');
  if (savedTheme) {
    applyAdminTheme(savedTheme);
  }
});

app.mount("#app");
