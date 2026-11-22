/** What the webview says about its WebGL; WebKitGTK may hide the real GPU. */
export interface WebGLProbe {
  webgl2: boolean;
  vendor: string;
  renderer: string;
  /** The renderer string names a CPU rasterizer such as llvmpipe. */
  software: boolean;
}

const softwareRenderers = /llvmpipe|softpipe|swiftshader|software/i;

export function probeWebGL(): WebGLProbe {
  const gl = document.createElement("canvas").getContext("webgl2");
  if (!gl) {
    return { webgl2: false, vendor: "", renderer: "", software: false };
  }
  const debug = gl.getExtension("WEBGL_debug_renderer_info");
  const vendor = String(gl.getParameter(debug ? debug.UNMASKED_VENDOR_WEBGL : gl.VENDOR));
  const renderer = String(gl.getParameter(debug ? debug.UNMASKED_RENDERER_WEBGL : gl.RENDERER));
  gl.getExtension("WEBGL_lose_context")?.loseContext();
  return { webgl2: true, vendor, renderer, software: softwareRenderers.test(renderer) };
}
