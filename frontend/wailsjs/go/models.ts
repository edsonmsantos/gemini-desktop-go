export namespace main {
	
	export class AppConfigResponse {
	    port: number;
	    baseUrl: string;
	    agent: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfigResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.baseUrl = source["baseUrl"];
	        this.agent = source["agent"];
	    }
	}

}

