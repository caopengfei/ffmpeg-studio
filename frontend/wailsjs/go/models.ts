export namespace main {
	
	export class CommandPreview {
	    steps: string[];
	    err: string;
	
	    static createFrom(source: any = {}) {
	        return new CommandPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.steps = source["steps"];
	        this.err = source["err"];
	    }
	}
	export class PickedFile {
	    path: string;
	    url: string;
	    name: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new PickedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.url = source["url"];
	        this.name = source["name"];
	        this.size = source["size"];
	    }
	}
	export class PreviewResult {
	    url: string;
	    isProxy: boolean;
	    note: string;
	    mediaInfo?: media.MediaInfo;
	
	    static createFrom(source: any = {}) {
	        return new PreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.isProxy = source["isProxy"];
	        this.note = source["note"];
	        this.mediaInfo = this.convertValues(source["mediaInfo"], media.MediaInfo);
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

export namespace media {
	
	export class FFmpegInfo {
	    available: boolean;
	    ffmpegPath: string;
	    ffprobePath: string;
	    version: string;
	    source: string;
	    tried: string[];
	    programDir: string;
	    downloadUrl: string;
	    incomplete: string;
	
	    static createFrom(source: any = {}) {
	        return new FFmpegInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffprobePath = source["ffprobePath"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.tried = source["tried"];
	        this.programDir = source["programDir"];
	        this.downloadUrl = source["downloadUrl"];
	        this.incomplete = source["incomplete"];
	    }
	}
	export class StreamInfo {
	    index: number;
	    codec: string;
	    profile: string;
	    width: number;
	    height: number;
	    fps: number;
	    bitRate: number;
	    sampleRate: number;
	    channels: number;
	    pixFmt: string;
	
	    static createFrom(source: any = {}) {
	        return new StreamInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.codec = source["codec"];
	        this.profile = source["profile"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.fps = source["fps"];
	        this.bitRate = source["bitRate"];
	        this.sampleRate = source["sampleRate"];
	        this.channels = source["channels"];
	        this.pixFmt = source["pixFmt"];
	    }
	}
	export class MediaInfo {
	    path: string;
	    format: string;
	    duration: number;
	    size: number;
	    bitRate: number;
	    hasVideo: boolean;
	    hasAudio: boolean;
	    video?: StreamInfo;
	    audio?: StreamInfo;
	    probeFailed: string;
	
	    static createFrom(source: any = {}) {
	        return new MediaInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.format = source["format"];
	        this.duration = source["duration"];
	        this.size = source["size"];
	        this.bitRate = source["bitRate"];
	        this.hasVideo = source["hasVideo"];
	        this.hasAudio = source["hasAudio"];
	        this.video = this.convertValues(source["video"], StreamInfo);
	        this.audio = this.convertValues(source["audio"], StreamInfo);
	        this.probeFailed = source["probeFailed"];
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

export namespace task {
	
	export class AudioSettings {
	    codec: string;
	    bitRate: string;
	    sampleRate: number;
	    channels: number;
	
	    static createFrom(source: any = {}) {
	        return new AudioSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.codec = source["codec"];
	        this.bitRate = source["bitRate"];
	        this.sampleRate = source["sampleRate"];
	        this.channels = source["channels"];
	    }
	}
	export class CompressSettings {
	    mode: string;
	    crf: number;
	    preset: string;
	    targetMb: number;
	    audioKbps: number;
	    copyAudio: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CompressSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.crf = source["crf"];
	        this.preset = source["preset"];
	        this.targetMb = source["targetMb"];
	        this.audioKbps = source["audioKbps"];
	        this.copyAudio = source["copyAudio"];
	    }
	}
	export class GifSettings {
	    start: number;
	    end: number;
	    fps: number;
	    width: number;
	    twoPass: boolean;
	    loop: number;
	
	    static createFrom(source: any = {}) {
	        return new GifSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	        this.fps = source["fps"];
	        this.width = source["width"];
	        this.twoPass = source["twoPass"];
	        this.loop = source["loop"];
	    }
	}
	export class ScaleSettings {
	    width: number;
	    height: number;
	    keepAspect: boolean;
	    flags: string;
	
	    static createFrom(source: any = {}) {
	        return new ScaleSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.keepAspect = source["keepAspect"];
	        this.flags = source["flags"];
	    }
	}
	export class SnapshotSettings {
	    time: number;
	    format: string;
	    quality: number;
	    batchEvery: number;
	    asCover: boolean;
	    coverImage: string;
	    outDir: string;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.format = source["format"];
	        this.quality = source["quality"];
	        this.batchEvery = source["batchEvery"];
	        this.asCover = source["asCover"];
	        this.coverImage = source["coverImage"];
	        this.outDir = source["outDir"];
	    }
	}
	export class WatermarkSettings {
	    items: watermark.Item[];
	    targetW: number;
	    targetH: number;
	
	    static createFrom(source: any = {}) {
	        return new WatermarkSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], watermark.Item);
	        this.targetW = source["targetW"];
	        this.targetH = source["targetH"];
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
	export class TrimSettings {
	    start: number;
	    end: number;
	    accurate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TrimSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	        this.accurate = source["accurate"];
	    }
	}
	export class VideoSettings {
	    codec: string;
	    crf: number;
	    preset: string;
	    bitRate: string;
	    maxRate: string;
	    bufSize: string;
	    pixFmt: string;
	    fps: string;
	
	    static createFrom(source: any = {}) {
	        return new VideoSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.codec = source["codec"];
	        this.crf = source["crf"];
	        this.preset = source["preset"];
	        this.bitRate = source["bitRate"];
	        this.maxRate = source["maxRate"];
	        this.bufSize = source["bufSize"];
	        this.pixFmt = source["pixFmt"];
	        this.fps = source["fps"];
	    }
	}
	export class Spec {
	    kind: string;
	    input: string;
	    output: string;
	    duration: number;
	    hasVideo: boolean;
	    hasAudio: boolean;
	    sourceW: number;
	    sourceH: number;
	    sourceFps: number;
	    sourceCodec: string;
	    video?: VideoSettings;
	    audio?: AudioSettings;
	    trim?: TrimSettings;
	    scale?: ScaleSettings;
	    compress?: CompressSettings;
	    snapshot?: SnapshotSettings;
	    gif?: GifSettings;
	    watermark?: WatermarkSettings;
	
	    static createFrom(source: any = {}) {
	        return new Spec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.duration = source["duration"];
	        this.hasVideo = source["hasVideo"];
	        this.hasAudio = source["hasAudio"];
	        this.sourceW = source["sourceW"];
	        this.sourceH = source["sourceH"];
	        this.sourceFps = source["sourceFps"];
	        this.sourceCodec = source["sourceCodec"];
	        this.video = this.convertValues(source["video"], VideoSettings);
	        this.audio = this.convertValues(source["audio"], AudioSettings);
	        this.trim = this.convertValues(source["trim"], TrimSettings);
	        this.scale = this.convertValues(source["scale"], ScaleSettings);
	        this.compress = this.convertValues(source["compress"], CompressSettings);
	        this.snapshot = this.convertValues(source["snapshot"], SnapshotSettings);
	        this.gif = this.convertValues(source["gif"], GifSettings);
	        this.watermark = this.convertValues(source["watermark"], WatermarkSettings);
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

export namespace watermark {
	
	export class Item {
	    kind: string;
	    enabled: boolean;
	    path: string;
	    text: string;
	    fontFile: string;
	    fontSizeRatio: number;
	    color: string;
	    opacity: number;
	    borderW: number;
	    borderColor: string;
	    box: boolean;
	    boxColor: string;
	    boxBorderW: number;
	    x: number;
	    y: number;
	    wRatio: number;
	    hasTime: boolean;
	    start: number;
	    end: number;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.enabled = source["enabled"];
	        this.path = source["path"];
	        this.text = source["text"];
	        this.fontFile = source["fontFile"];
	        this.fontSizeRatio = source["fontSizeRatio"];
	        this.color = source["color"];
	        this.opacity = source["opacity"];
	        this.borderW = source["borderW"];
	        this.borderColor = source["borderColor"];
	        this.box = source["box"];
	        this.boxColor = source["boxColor"];
	        this.boxBorderW = source["boxBorderW"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.wRatio = source["wRatio"];
	        this.hasTime = source["hasTime"];
	        this.start = source["start"];
	        this.end = source["end"];
	    }
	}

}

