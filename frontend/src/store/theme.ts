import { defineStore } from "pinia";
import { ref, watch } from "vue";

export type ThemeMode = "light" | "dark" | "system";

export const useThemeStore = defineStore("theme", () => {
  const mode = ref<ThemeMode>(
    (localStorage.getItem("theme-mode") as ThemeMode) || "light"
  );
  const isDark = ref(false);

  const updateTheme = () => {
    let shouldBeDark = false;

    if (mode.value === "system") {
      shouldBeDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    } else {
      shouldBeDark = mode.value === "dark";
    }

    isDark.value = shouldBeDark;

    if (shouldBeDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  };

  const setMode = (newMode: ThemeMode) => {
    mode.value = newMode;
    localStorage.setItem("theme-mode", newMode);
    updateTheme();
  };

  const toggleTheme = () => {
    const newMode = isDark.value ? "light" : "dark";
    setMode(newMode);
  };

  const init = () => {
    updateTheme();

    if (mode.value === "system") {
      window
        .matchMedia("(prefers-color-scheme: dark)")
        .addEventListener("change", updateTheme);
    }
  };

  watch(mode, updateTheme);

  return {
    mode,
    isDark,
    setMode,
    toggleTheme,
    init,
  };
});
