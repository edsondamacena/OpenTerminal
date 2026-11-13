// Ícones do OpenTerminal: todos do Lucide, pelo wrapper `icon` (FR-169).
import {
  ArrowLeft,
  CircleAlert,
  CircleCheck,
  CircleMinus,
  LoaderCircle,
  Unplug,
  Info,
  Palette,
  ShieldCheck,
  Cloud,
  SlidersHorizontal,
  Network,
  CaseSensitive,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ClipboardCopy,
  ClipboardPaste,
  Keyboard,
  Moon,
  Pencil,
  Regex,
  Copy,
  Ellipsis,
  Eye,
  EyeOff,
  File,
  Folder,
  FolderOpen,
  KeyRound,
  Lock,
  LockOpen,
  Minus,
  Trash2,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  RefreshCw,
  Search,
  Server,
  Settings,
  Square,
  SquareTerminal,
  Terminal,
  Router,
  Cable,
  EthernetPort,
  Shield,
  RadioTower,
  Wifi,
  Globe,
  Waypoints,
  TriangleAlert,
  Upload,
  Download,
  ArrowLeftRight,
  X,
} from 'lucide-react';
import { icon } from './Icon';

export const ServerIcon = icon(Server);
export const FolderIcon = icon(Folder);
export const FolderOpenIcon = icon(FolderOpen);
export const TerminalIcon = icon(SquareTerminal);
export const FileIcon = icon(File);
export const UploadIcon = icon(Upload);
export const DownloadIcon = icon(Download);
export const LockIcon = icon(Lock);
export const UnlockIcon = icon(LockOpen);
export const PlusIcon = icon(Plus);
export const SearchIcon = icon(Search);
export const CloseIcon = icon(X);
export const MoreIcon = icon(Ellipsis);
export const KeyIcon = icon(KeyRound);
export const ForwardIcon = icon(ArrowLeftRight);
export const SettingsIcon = icon(Settings);
export const WarningIcon = icon(TriangleAlert);
export const RefreshIcon = icon(RefreshCw);
export const EyeIcon = icon(Eye);
export const EyeOffIcon = icon(EyeOff);
export const ChevronIcon = icon(ChevronRight);
export const ChevronLeftIcon = icon(ChevronLeft);

// Barra de título e barra lateral.
export const MinimiseIcon = icon(Minus);
export const MaximiseIcon = icon(Square);
export const RestoreIcon = icon(Copy);
export const SidebarCollapseIcon = icon(PanelLeftClose);
export const SidebarExpandIcon = icon(PanelLeftOpen);
export const CaseSensitiveIcon = icon(CaseSensitive);
export const RegexIcon = icon(Regex);
export const ChevronUpIcon = icon(ChevronUp);
export const ChevronDownIcon = icon(ChevronDown);
export const PasteIcon = icon(ClipboardPaste);
export const KeyboardIcon = icon(Keyboard);
export const ThemeIcon = icon(Moon);
export const RenameIcon = icon(Pencil);
export const CheckIcon = icon(Check);
export const CopyIcon = icon(ClipboardCopy);
export const MinusIcon = icon(Minus);
export const TrashIcon = icon(Trash2);
export const BackIcon = icon(ArrowLeft);
export const InfoIcon = icon(Info);
export const PaletteIcon = icon(Palette);
export const ShieldIcon = icon(ShieldCheck);
export const CloudIcon = icon(Cloud);
export const GeneralIcon = icon(SlidersHorizontal);
export const NetworkIcon = icon(Network);
// Equipamentos de rede no avatar do host (ui/deviceIcons.tsx).
export const RouterIcon = icon(Router);
export const SwitchIcon = icon(Network);
export const OltIcon = icon(Cable);
export const OnuIcon = icon(EthernetPort);
export const FirewallIcon = icon(Shield);
export const RadioIcon = icon(RadioTower);
export const WifiIcon = icon(Wifi);
export const EdgeIcon = icon(Globe);
export const BngIcon = icon(Waypoints);
export const SshIcon = icon(SquareTerminal);
export const TelnetIcon = icon(Terminal);
export const ConnectedIcon = icon(CircleCheck);
export const ConnectingIcon = icon(LoaderCircle);
export const DisconnectedIcon = icon(Unplug);
export const ErrorIcon = icon(CircleAlert);
export const ExitedIcon = icon(CircleMinus);
