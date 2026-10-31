export declare const SSHD: { host: string; port: number };
export declare const SSHD_LEGACY: { host: string; port: number };
export declare const IDLE_KILLER: { host: string; port: number };
export declare function dockerAvailable(): boolean;
export declare function waitForSshd(
  timeoutMs?: number,
  target?: { host: string; port: number },
): Promise<void>;
export declare function up(): Promise<void>;
export declare function down(): void;
export declare function stop(): void;
export declare function start(): Promise<void>;
export declare function rotateKey(): Promise<void>;
export declare function restoreKey(): Promise<void>;
