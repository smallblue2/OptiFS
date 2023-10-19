# School of Computing CA326 Year 3 Project Proposal

### Project Title
OptiFS - A Modern Day Optimised Network File Management System

### Student 1
Name: Zoe Collins \
ID: 21503159

### Student 2
Name: Niall Ryan \
ID: 21454746

### Staff Member Consulted
Prof. Stephen Blott

## Project Description

### What is it?
OptiFS, a modern day solution specifically for large groups of machines accessing a shared file system through Network File System (NFS).

Computer labs contain clusters of computers, and when user files are managed by shared file systems like NFS, storing all this data can become a very large, costly and impractical task. Slow disk operations are a notoriously frustrating experience amongst students during exams, labs and any situation where many shared machines are powered on and running at the same time.

Furthermore as a student, it can be a jarring experience having to physically attend computer labs to retrieve your work or data. Being able to view and download your files on the go means that you can access your files anywhere and at any time.

OptiFS would deliver performance by massively reducing the file count on shared file systems through deduplication and give users additional interfaces to access their information where they may need.

We would innovate an increasingly difficult and logistically challenging problem introduced by managing many users on many machines by combining elements of systems programming, web development and database management.

![A preliminary diagram of the systems and architecture in place](OptiFS_Diagram.pdf)

### Concept Ideas
**Content-Based Hashing** will be used to store data based on its content rather than its name or location. This will be used to reduce duplicate data stored between all users as a whole.

**Garbage Collection** will be utilised to count references to files in our system. Files that no longer have references will be deleted from the file system.

**False namespaces** will be employed through symbolic links. Users will believe they have individual and unique files in their ownership.

**User Web Interface** likely implemented through websockets to fetch file content without reloading the page. Viewing content will differ depending on file/media format (e.g. txt, mp3, mp4, png, pdf) and users will be able to download files directly.

**Optimizations** will include implementation of a caching system for frequently accessed files to avoid constantly having to query the storage system every time. If time allows, we could also introduce “chunking” for very large files so if two large files differ slightly, we wouldn’t have to store two almost identical files.

## Division of Work

### Scrum Planning
We have decided to set up sprints using Jira for a scrum development lifecycle. We will lay out the work that needs to be done during weeklong sprints, allocating work at the time. We feel that this is a good method to assign tasks on an adhoc basis, and is an excellent tool to make sure that nobody is taking on too much of the workload.

### Strength-Based Task Assignment
While we are both interested in our own topics, we will try to assign work where we think suits best, delving into both sides of development.

Typically speaking, Niall leans more towards lower level programming as he has more experience with operating systems concepts, systems programming languages, file systems, shell scripting and more. Loosely speaking, he would be more comfortable covering the:
- Libfuse virtual file system 
- Postgres database management

Zoe has a passion for frontend development, with a keen eye for design. She is proficient in all aspects of web development, from development frameworks to UI components. She would be most comfortable covering the:
- SvelteKit frontend interface
- Django backend

## Programming Languages

### Systems Programming
- Go
- C

### Web Development
- Javascript / Typescript
- Python

### Database Management
- SQL

## Programming Tools

### Systems Programming
- Libfuse
- Make

### Web Development
- SvelteKit
- Django
- Tailwindcss

### Database
- Postgres

### Containerization
- Docker

### Version Control
- Git

## Learning Challenges

### New Technologies
We find this project to be a great opportunity to expand our horizons in terms of technology. Branching out into newfi-wave languages like Golang and frontend frameworks like SvelteKit is something that we have both wanted to do for a long time, and there is no better chance than the present.

### How to Build a Virtual File System
Libfuse is a completely new concept for both parties. We aim to explore how virtual file systems are used and implemented, and how they can be optimised in the context of file sharing.

### How to Implement Garbage Collection
This is a major part of our project where we will learn how to discover which files are no longer in use, and therefore disposable.

### How to Implement User Authentication
We will need to learn to store and link usernames to passwords whilst keeping security heavily in mind due to the heavy data-centric focus of our project. This is something that we have not encountered before.

### Synchronisation and Scalability
Naturally being a shared file system with multiple interfaces, we will have to implement parallelism to serve and deal with all requests. This will introduce synchronisation issues and race conditions that will have to be taken into account for.

### How to Optimise
Naturally being a shared file system with multiple interfaces, we will have to implement parallelism to serve and deal with all requests. This will introduce synchronisation issues and race conditions that will have to be taken into account for.

### Testing Reliability
As we are dealing with files and people’s data, we need to test rigorously to make sure that we are not deleting information that isn’t meant to be removed. This will have a big learning curve.

## Hardware / Software Platforms

### Libfuse and NFS
Linux-exclusive for PC machines.

### Web Interface
Any mainstream browser on desktop or mobile devices

## Special Hardware / Software Requirements
Host machine must have Docker installed, as our software and dependencies will be packaged with it.
