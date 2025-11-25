// Static probe dictionary for KernelXRay educational content

export interface ProbeInfo {
  id: string
  name: string
  tracepoint: string
  description: string
  sourceCode: string
  kernelStructs: string[]
  category: 'process' | 'file' | 'network'
}

export const probes: ProbeInfo[] = [
  {
    id: 'exec',
    name: 'Process Execution Monitor',
    tracepoint: 'tp/sched/sched_process_exec',
    description: 'Monitors all process execution events. When any process calls execve() system call, this probe triggers and captures the new process PID, parent process information, command name, and Cgroup ID for container detection.',
    sourceCode: `SEC("tp/sched/sched_process_exec")
int handle_exec(struct trace_event_raw_sched_process_exec* ctx) {
    struct exec_event* event;
    struct task_struct* task = (struct task_struct*)bpf_get_current_task_btf();
    
    event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;
    
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->ppid = BPF_CORE_READ(task, real_parent, tgid);
    event->cgroup_id = bpf_get_current_cgroup_id();
    bpf_get_current_comm(&event->comm, sizeof(event->comm));
    
    // Read parent command name
    struct task_struct* parent = BPF_CORE_READ(task, real_parent);
    bpf_probe_read_kernel_str(&event->pcomm, sizeof(event->pcomm), 
                              BPF_CORE_READ(parent, comm));
    
    bpf_ringbuf_submit(event, 0);
    return 0;
}`,
    kernelStructs: [
      'task_struct.pid',
      'task_struct.tgid', 
      'task_struct.real_parent',
      'task_struct.comm[16]'
    ],
    category: 'process'
  },
  {
    id: 'openat',
    name: 'File Access Monitor',
    tracepoint: 'tp/syscalls/sys_enter_openat',
    description: 'Monitors file access operations through the openat() system call. Captures the file path, access flags, and process information. Essential for detecting unauthorized file access patterns.',
    sourceCode: `SEC("tp/syscalls/sys_enter_openat")
int handle_openat(struct trace_event_raw_sys_enter* ctx) {
    struct file_event* event;
    
    event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;
    
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->cgroup_id = bpf_get_current_cgroup_id();
    
    // Read flags from syscall args
    event->flags = (u32)ctx->args[2];
    
    // Read filename from userspace
    const char* filename = (const char*)ctx->args[1];
    bpf_probe_read_user_str(&event->filename, sizeof(event->filename), filename);
    
    bpf_ringbuf_submit(event, 0);
    return 0;
}`,
    kernelStructs: [
      'trace_event_raw_sys_enter.args[]',
      'task_struct.pid',
      'cgroup_id'
    ],
    category: 'file'
  },
  {
    id: 'connect',
    name: 'Network Connection Monitor',
    tracepoint: 'tp/syscalls/sys_enter_connect',
    description: 'Monitors outbound network connections via the connect() system call. Captures destination IP address, port, protocol family (IPv4/IPv6), and the initiating process. Critical for detecting C2 callbacks and data exfiltration.',
    sourceCode: `SEC("tp/syscalls/sys_enter_connect")
int handle_connect(struct trace_event_raw_sys_enter* ctx) {
    struct connect_event* event;
    struct sockaddr_in* addr4;
    struct sockaddr_in6* addr6;
    
    event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;
    
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->cgroup_id = bpf_get_current_cgroup_id();
    
    // Read socket address from userspace
    struct sockaddr* addr = (struct sockaddr*)ctx->args[1];
    bpf_probe_read_user(&event->family, sizeof(event->family), &addr->sa_family);
    
    if (event->family == AF_INET) {
        addr4 = (struct sockaddr_in*)addr;
        bpf_probe_read_user(&event->port, sizeof(event->port), &addr4->sin_port);
        bpf_probe_read_user(&event->addr_v4, sizeof(event->addr_v4), &addr4->sin_addr);
    } else if (event->family == AF_INET6) {
        addr6 = (struct sockaddr_in6*)addr;
        bpf_probe_read_user(&event->port, sizeof(event->port), &addr6->sin6_port);
        bpf_probe_read_user(&event->addr_v6, sizeof(event->addr_v6), &addr6->sin6_addr);
    }
    
    event->port = __builtin_bswap16(event->port); // Network to host byte order
    
    bpf_ringbuf_submit(event, 0);
    return 0;
}`,
    kernelStructs: [
      'sockaddr.sa_family',
      'sockaddr_in.sin_port',
      'sockaddr_in.sin_addr',
      'sockaddr_in6.sin6_port',
      'sockaddr_in6.sin6_addr'
    ],
    category: 'network'
  }
]

// Kernel structure information for educational display
export interface KernelStruct {
  name: string
  description: string
  fields: { name: string; type: string; description: string }[]
}

export const kernelStructs: KernelStruct[] = [
  {
    name: 'task_struct',
    description: 'The fundamental process descriptor in Linux. Contains all information about a process/thread.',
    fields: [
      { name: 'pid', type: 'pid_t', description: 'Process ID (thread ID)' },
      { name: 'tgid', type: 'pid_t', description: 'Thread Group ID (process ID)' },
      { name: 'real_parent', type: 'struct task_struct*', description: 'Pointer to parent process' },
      { name: 'comm[16]', type: 'char[16]', description: 'Executable name (max 16 chars)' },
      { name: 'cgroups', type: 'struct css_set*', description: 'Control group membership' }
    ]
  },
  {
    name: 'sockaddr_in',
    description: 'IPv4 socket address structure used in network syscalls.',
    fields: [
      { name: 'sin_family', type: 'sa_family_t', description: 'Address family (AF_INET)' },
      { name: 'sin_port', type: '__be16', description: 'Port number (network byte order)' },
      { name: 'sin_addr', type: 'struct in_addr', description: 'IPv4 address' }
    ]
  }
]

