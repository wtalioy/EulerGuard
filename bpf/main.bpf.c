#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define TASK_COMM_LEN 16
#define PATH_MAX_LEN 256
#define EVENT_TYPE_EXEC 1
#define EVENT_TYPE_FILE_OPEN 2

struct exec_event {
    u8 type;
    u32 pid;
    u32 ppid;
    u64 cgroup_id;
    char comm[TASK_COMM_LEN];
    char pcomm[TASK_COMM_LEN];
} __attribute__((packed));

struct file_open_event {
    u8 type;
    u32 pid;
    u64 cgroup_id;
    u32 flags;
    char filename[PATH_MAX_LEN];
} __attribute__((packed));

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

// Map to store monitored file paths (configured from userspace)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, char[PATH_MAX_LEN]);
    __type(value, u8);
} monitored_paths SEC(".maps");

// Per-CPU array for temporary path buffer (avoids stack overflow)
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, char[PATH_MAX_LEN]);
} path_buffer SEC(".maps");

static __always_inline u32 get_parent_pid(struct task_struct* task)
{
    if (!task)
        return 0;

    return BPF_CORE_READ(task, real_parent, tgid);
}

SEC("tp/sched/sched_process_exec")
int handle_exec(struct trace_event_raw_sched_process_exec* ctx)
{
    struct exec_event* event;
    struct task_struct* task = (struct task_struct*)bpf_get_current_task_btf();
    struct task_struct* parent;
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;

    event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->type = EVENT_TYPE_EXEC;
    event->pid = pid;
    event->ppid = get_parent_pid(task);
    event->cgroup_id = bpf_get_current_cgroup_id();
    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    // Get parent process name
    parent = BPF_CORE_READ(task, real_parent);
    if (parent) {
        BPF_CORE_READ_STR_INTO(&event->pcomm, parent, comm);
    } else {
        event->pcomm[0] = '\0';
    }

    bpf_ringbuf_submit(event, 0);
    return 0;
}

// Helper to check if path should be monitored
static __always_inline bool should_monitor_path(char* path_buf)
{
    // Check for exact match in monitored_paths map
    u8* value = bpf_map_lookup_elem(&monitored_paths, path_buf);
    if (value && *value)
        return true;

    // Check prefix matches at directory boundaries
    // Simple approach: check common prefixes like /etc/, /home/, /root/
    // The map lookup will handle the actual prefix matching

    // For paths like /etc/passwd, check prefixes: /etc/, /etc, /
    // Userspace populates the map with directory prefixes ending in '/'
    for (int i = 1; i < 64 && i < PATH_MAX_LEN; i++) {
        if (path_buf[i] == '\0')
            break;

        // At directory boundaries, check if this prefix is monitored
        if (path_buf[i] == '/') {
            // Temporarily null-terminate at boundary
            char saved = path_buf[i + 1];
            path_buf[i + 1] = '\0';

            u8* prefix_value = bpf_map_lookup_elem(&monitored_paths, path_buf);

            // Restore the character
            path_buf[i + 1] = saved;

            if (prefix_value && *prefix_value)
                return true;
        }
    }

    return false;
}

// Tracepoint for sys_enter_openat
SEC("tp/syscalls/sys_enter_openat")
int tracepoint_openat(struct trace_event_raw_sys_enter* ctx)
{
    struct file_open_event* event;
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;

    // Get filename from syscall args (args[1] for openat)
    const char* filename = (const char*)ctx->args[1];

    // Quick check before allocating event - only monitor absolute paths
    char first_char;
    bpf_probe_read_user(&first_char, 1, filename);
    if (first_char != '/')
        return 0;

    // Get per-cpu buffer for path (avoids stack overflow)
    u32 key = 0;
    char* path_buf = bpf_map_lookup_elem(&path_buffer, &key);
    if (!path_buf)
        return 0;

    // Read filename from userspace into per-cpu buffer
    long ret = bpf_probe_read_user_str(path_buf, PATH_MAX_LEN, filename);
    if (ret <= 0)
        return 0;

    // Filter: only send events for monitored paths
    if (!should_monitor_path(path_buf))
        return 0;

    event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->type = EVENT_TYPE_FILE_OPEN;
    event->pid = pid;
    event->cgroup_id = bpf_get_current_cgroup_id();
    event->flags = (u32)ctx->args[2]; // flags argument

    // Copy already-read filename to event
    __builtin_memcpy(event->filename, path_buf, PATH_MAX_LEN);

    bpf_ringbuf_submit(event, 0);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
