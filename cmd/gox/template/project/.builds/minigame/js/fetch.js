class Headers {
  constructor(init = {}) {
    // Use a clean object to store header data
    this.map = Object.create(null);

    if (init instanceof Headers) {
      // If the passed init is a Headers instance, copy its content
      init.forEach((value, key) => {
        this.append(key, value);
      });
    } else if (typeof init === 'object' && init !== null) {
      // If passed init is a plain object, add its key-value pairs to Headers
      Object.keys(init).forEach(key => {
        this.append(key, init[key]);
      });
    }
  }

  // Add header, if already exists merge into comma-separated string
  append(name, value) {
    name = name.toLowerCase();
    if (this.map[name]) {
      this.map[name] += ', ' + value;
    } else {
      this.map[name] = String(value);
    }
  }

  // Set header, directly overwrite old value
  set(name, value) {
    this.map[name.toLowerCase()] = String(value);
  }

  // Get header value, return null if doesn't exist
  get(name) {
    return this.map[name.toLowerCase()] || null;
  }

  // Check if header exists
  has(name) {
    return Object.prototype.hasOwnProperty.call(this.map, name.toLowerCase());
  }

  // Delete specified header
  delete(name) {
    delete this.map[name.toLowerCase()];
  }

  // Iterate through all headers
  forEach(callback) {
    for (let key in this.map) {
      callback(this.map[key], key, this);
    }
  }
}

// Simulated ReadableStream (simplified version)
// Here only returns a chunk containing the entire body content
class SimulatedReadableStream {
  constructor(content) {
    this._content = content;
    this.locked = false;
  }
  getReader() {
    if (this.locked) {
      throw new Error("Stream is already locked");
    }
    this.locked = true;
    let done = false;
    return {
      read: () => {
        if (!done) {
          done = true;
          return Promise.resolve({ value: this._content, done: false });
        } else {
          return Promise.resolve({ done: true });
        }
      },
      releaseLock: () => {
        this.locked = false;
      },
      cancel: () => {
        this.locked = false;
        return Promise.resolve();
      }
    };
  }
}

// Simulated Response class
class Response {
  /**
   * @param {*} body Response body content, can be string, object, or ArrayBuffer etc.
   * @param {Object} options Configuration options
   *   - status: HTTP status code, defaults to 200
   *   - statusText: Status description, defaults to 'OK'
   *   - headers: Response headers object
   *   - url: Response URL
   */
  constructor(body, options = {}) {
    this._bodyContent = body;
    this.status = options.status || 200;
    this.statusText = options.statusText || 'OK';
    this.headers = new Headers(options.headers);
    this.url = options.url || '';
    this.ok = this.status >= 200 && this.status < 300;

    // Mark whether body has been consumed
    this.bodyUsed = false;

    // Simulate body property: if has body create a readable stream
    if (body != null) {
      let content;
      if (typeof body === 'string') {
        content = new TextEncoder().encode(body);
      } else if (body instanceof ArrayBuffer) {
        content = new Uint8Array(body)
      } else if (typeof body === 'object') {
        // Convert object to JSON string
        const _content = JSON.stringify(body);
        content = new TextEncoder().encode(_content)
      } else {
        content = new TextEncoder().encode(_content);
      }
      this.body = new SimulatedReadableStream(content);
    } else {
      this.body = null;
    }
  }

  // Internal method: consume body, ensure can only be read once
  _consumeBody() {
    if (this.bodyUsed) {
      return Promise.reject(new TypeError("Body has already been consumed."));
    }
    this.bodyUsed = true;
    if (!this.body) {
      return Promise.resolve('');
    }
    const reader = this.body.getReader();
    // Here assume stream only returns entire content once
    return reader.read().then(result => {
      reader.releaseLock();
      return result.value || '';
    });
  }

  /**
   * Parse response body as text, returns Promise
   */
  text() {
    return this._consumeBody();
  }

  /**
   * Parse response body as JSON object, returns Promise
   */
  json() {
    return this.text().then(text => {
      try {
        return JSON.parse(text);
      } catch (error) {
        return Promise.reject(new Error("Invalid JSON: " + error.message));
      }
    });
  }

  /**
   * Convert response body to ArrayBuffer, returns Promise
   */
  arrayBuffer() {
    if (this.bodyUsed) {
      return Promise.reject(new TypeError("Body has already been consumed."));
    }
    this.bodyUsed = true;
    
    if (!this.body) {
      return Promise.resolve(new ArrayBuffer(0));
    }
    
    // If _bodyContent is already ArrayBuffer, return directly
    if (this._bodyContent instanceof ArrayBuffer) {
      return Promise.resolve(this._bodyContent);
    }
    
    // Otherwise convert from text
    return this.text().then(text => {
      if (typeof text === 'string') {
        const buffer = new ArrayBuffer(text.length);
        const view = new Uint8Array(buffer);
        for (let i = 0; i < text.length; i++) {
          view[i] = text.charCodeAt(i);
        }
        return buffer;
      }
      return text; // May already be ArrayBuffer
    });
  }
}


function Fetch(url, options = {}) {
  return new Promise((resolve, reject) => {
      // Safely handle headers, support multiple formats
      let headers = {};
      if (options.headers) {
        if (Array.isArray(options.headers)) {
          // If array format [['key', 'value'], ...]
          headers = options.headers.reduce((obj, [key, value]) => {
            obj[key] = value;
            return obj;
          }, {});
        } else if (options.headers instanceof Headers) {
          // If Headers instance
          options.headers.forEach((value, key) => {
            headers[key] = value;
          });
        } else if (typeof options.headers === 'object') {
          // If plain object
          headers = { ...options.headers };
        }
      }

      // Determine if it's local file or network file
      const isNetworkUrl = url.startsWith('http://') || url.startsWith('https://');
      
      if (isNetworkUrl) {
        // Network file, use wx.request
        const responseType = headers["Accept"] === "application/octet-stream" ? "arraybuffer" : "text";
        const dataType = headers["Content-Type"] === "application/json" ? "json" : ""
        wx.request({
            url,
            method: options.method || 'GET',
            data: options.body || {},
            header: headers,
            dataType: dataType,
            responseType: responseType,
            success(res) {
                const response = new Response(res.data, {
                  status: res.statusCode,
                  statusText: res.errMsg,
                  headers: res.header,
                });
                resolve(response);
            },
            fail(err) {
                reject(err);
            }
        });
      } else {
        // Local file, use file system API
        wx.getFileSystemManager().readFile({
          filePath: url,
          success(res) {
            const response = new Response(res.data, {
              status: 200,
              statusText: 'OK',
              headers: {},
            });
            resolve(response);
          },
          fail(err) {
            console.error('Failed to read local file:', err);
            reject(new Error(`Failed to read local file: ${err.errMsg}`));
          }
        });
      }
  });
}

// Check global environment and mount fetch
GameGlobal.fetch = Fetch;
GameGlobal.Headers = Headers;
GameGlobal.Response = Response;