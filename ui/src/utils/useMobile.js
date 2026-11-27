import { useSyncExternalStore } from "react";

const phoneViewport = window.matchMedia("(max-width: 767px)");

function subscribe(onChange) {
  phoneViewport.addEventListener("change", onChange);
  return () => phoneViewport.removeEventListener("change", onChange);
}

export default function useMobile() {
  return useSyncExternalStore(subscribe, () => phoneViewport.matches);
}
