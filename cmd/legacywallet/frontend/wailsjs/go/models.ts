export namespace main {
	
	export class LaunchpadSettings {
	    apiUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new LaunchpadSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiUrl = source["apiUrl"];
	    }
	}
	export class NetworkSettings {
	    mode: string;
	    nodes: string[];
	
	    static createFrom(source: any = {}) {
	        return new NetworkSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.nodes = source["nodes"];
	    }
	}
	export class NodeTestResult {
	    node: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = source["node"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class Settings {
	    dataDir: string;
	    startNodeOnLaunch: boolean;
	    stopNodeOnExit: boolean;
	    defaultThreads: number;
	    defaultMiningAddress: string;
	    theme: string;
	    network: NetworkSettings;
	    launchpad: LaunchpadSettings;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataDir = source["dataDir"];
	        this.startNodeOnLaunch = source["startNodeOnLaunch"];
	        this.stopNodeOnExit = source["stopNodeOnExit"];
	        this.defaultThreads = source["defaultThreads"];
	        this.defaultMiningAddress = source["defaultMiningAddress"];
	        this.theme = source["theme"];
	        this.network = this.convertValues(source["network"], NetworkSettings);
	        this.launchpad = this.convertValues(source["launchpad"], LaunchpadSettings);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace nodeservice {
	
	export class Status {
	    running: boolean;
	    starting: boolean;
	    error?: string;
	    data_dir: string;
	    uptime_seconds: number;
	    stopping: boolean;
	    internal_node_pid?: number;
	    wallet_owned: boolean;
	    last_start_error?: string;
	    last_stop_error?: string;
	    rpc_port_in_use: boolean;
	    rpc_port_state?: string;
	    rpc_port_message?: string;
	    rpc_port_chain_id?: string;
	    rpc_port_pid?: number;
	    rpc_port_process?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.starting = source["starting"];
	        this.error = source["error"];
	        this.data_dir = source["data_dir"];
	        this.uptime_seconds = source["uptime_seconds"];
	        this.stopping = source["stopping"];
	        this.internal_node_pid = source["internal_node_pid"];
	        this.wallet_owned = source["wallet_owned"];
	        this.last_start_error = source["last_start_error"];
	        this.last_stop_error = source["last_stop_error"];
	        this.rpc_port_in_use = source["rpc_port_in_use"];
	        this.rpc_port_state = source["rpc_port_state"];
	        this.rpc_port_message = source["rpc_port_message"];
	        this.rpc_port_chain_id = source["rpc_port_chain_id"];
	        this.rpc_port_pid = source["rpc_port_pid"];
	        this.rpc_port_process = source["rpc_port_process"];
	    }
	}

}

