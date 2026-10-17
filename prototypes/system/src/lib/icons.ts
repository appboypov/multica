import AppWindow from "@lucide/svelte/icons/app-window";
import Box from "@lucide/svelte/icons/box";
import Camera from "@lucide/svelte/icons/camera";
import Database from "@lucide/svelte/icons/database";
import DoorOpen from "@lucide/svelte/icons/door-open";
import FileCode from "@lucide/svelte/icons/file-code";
import FlaskConical from "@lucide/svelte/icons/flask-conical";
import Folder from "@lucide/svelte/icons/folder";
import Globe from "@lucide/svelte/icons/globe";
import List from "@lucide/svelte/icons/list";
import ListTodo from "@lucide/svelte/icons/list-todo";
import MapIcon from "@lucide/svelte/icons/map";
import MapPin from "@lucide/svelte/icons/map-pin";
import MousePointerClick from "@lucide/svelte/icons/mouse-pointer-click";
import Package from "@lucide/svelte/icons/package";
import Play from "@lucide/svelte/icons/play";
import ScrollText from "@lucide/svelte/icons/scroll-text";
import Send from "@lucide/svelte/icons/send";
import Server from "@lucide/svelte/icons/server";
import Settings from "@lucide/svelte/icons/settings";
import Shapes from "@lucide/svelte/icons/shapes";
import Tag from "@lucide/svelte/icons/tag";
import Terminal from "@lucide/svelte/icons/terminal";
import User from "@lucide/svelte/icons/user";
import Workflow from "@lucide/svelte/icons/workflow";
import Zap from "@lucide/svelte/icons/zap";
import type { IconName } from "./system.ts";

/** The Lucide icon behind each icon name system.json may use. */
export const icons: Record<IconName, typeof Box> = {
  "app-window": AppWindow,
  box: Box,
  camera: Camera,
  database: Database,
  "door-open": DoorOpen,
  "file-code": FileCode,
  "flask-conical": FlaskConical,
  folder: Folder,
  globe: Globe,
  list: List,
  "list-todo": ListTodo,
  map: MapIcon,
  "map-pin": MapPin,
  "mouse-pointer-click": MousePointerClick,
  package: Package,
  play: Play,
  "scroll-text": ScrollText,
  send: Send,
  server: Server,
  settings: Settings,
  shapes: Shapes,
  tag: Tag,
  terminal: Terminal,
  user: User,
  workflow: Workflow,
  zap: Zap,
};
