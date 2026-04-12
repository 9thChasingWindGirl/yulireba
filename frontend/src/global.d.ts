// Global type declarations

interface Window {
  go?: {
    main?: {
      App?: {
        GetAnnouncement: () => Promise<string>;
        Open: (arg1: string) => Promise<void>;
        ProxyList: () => Promise<string[]>;
        Switch: (arg1: boolean, arg2: string, arg3: boolean) => Promise<string>;
        Version: () => Promise<string>;
        LoadSubscription: (arg1: string) => Promise<any[]>;
        LoadLocalFile: (arg1: string) => Promise<any[]>;
        GetAccountStatus: () => Promise<string>;
      };
    };
  };
}