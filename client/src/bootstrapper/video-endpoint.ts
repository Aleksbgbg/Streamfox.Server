import { VideoEndpoint } from "@/endpoints/video-endpoint";
import { VideoLister } from "@/endpoints/video-lister";
import { VideoUploader } from "@/endpoints/video-uploader";

const videoEndpoint = new VideoEndpoint();

export const videoLister: VideoLister = videoEndpoint;
export const videoUploader: VideoUploader = videoEndpoint;